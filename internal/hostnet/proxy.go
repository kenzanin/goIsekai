package hostnet

import (
	nethttp "net/http"
	"sync"
	"time"

	http "github.com/bogdanfinn/fhttp"
	tls_client "github.com/bogdanfinn/tls-client"
)

// Proxy is a sandboxed HTTP client that enforces standard header injection and
// per-plugin cookie persistence. Plugins must not open sockets directly; all
// network access flows through here.
type Proxy struct {
	mu               sync.Mutex
	defaultHeaders   map[string]string
	clients          map[string]tls_client.HttpClient // keyed by pluginID+"\x00"+profile
	uaOverrides      map[string]string                // per-plugin User-Agent override
	secCHUAExplicit  string                           // config-set Sec-CH-UA hint ("" = derive from UA)
	pendingVerify    map[string]verifySeed            // cookie jar seeds awaiting client creation
	needsJS          map[string]bool                  // per-plugin needs_js hint
	manualCookies    map[string]bool                  // per-plugin manual_cookies hint
	pins             map[string]string                // pluginID -> pinned profile ("stdlib" = use doRequestStd)
	hints            map[string][]string              // pluginID -> ordered profile ladder from metadata
	persistPin       func(pluginID, profile string)   // persists pin to DB; nil-safe
	cdp              CDPConfig
	pluginCDPTimeout map[string]time.Duration // per-plugin CDP solve budget (PLUGIN timeout key)

	// solveChallenge is swappable for tests; nil means the real rod solver.
	solveChallenge func(cfg CDPConfig, url string) ([]*http.Cookie, string, error)
	// stdlibTransport is shared by all stdlib requests for connection pooling.
	stdlibTransport *nethttp.Transport
	// submitSolve runs a challenge solver off the hot path. Defaults to a bare
	// goroutine so hostnet stays decoupled from the workers pool; main wires
	// it to LaneCDP (anti-bot solve lane) to cap concurrency.
	SubmitSolve func(f func())
}

// SetSubmitSolve overrides the default bare-goroutine solver dispatch.
// main wires it to workers.LaneCDP after the pool is constructed.
func (p *Proxy) SetSubmitSolve(fn func(f func())) {
	p.SubmitSolve = fn
}

// defaultUA is a browser-like User-Agent so requests are less likely to be
// blocked by anti-bot measures.
const defaultUA = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0 Safari/537.36"

// secCHUA is the Sec-CH-UA client hint matching defaultUA's Chrome brand.
// Kept in sync manually when defaultUA changes.
const secCHUA = `"Chromium";v="126", "Google Chrome";v="126", "Not-A.Brand";v="99"`

// defaultHeaderOrder fixes the order in which default headers are applied so
// the derived HeaderOrderKey is deterministic regardless of map iteration.
var defaultHeaderOrder = []string{"User-Agent", "Accept-Language", "Referer", "Sec-CH-UA"}

// NewProxy initializes a Proxy with browser-like default headers and an empty
// per-plugin client map. Each client is created lazily on first use.
func NewProxy() *Proxy {
	return &Proxy{
		defaultHeaders: map[string]string{
			"User-Agent":      defaultUA,
			"Accept-Language": "en-US,en;q=0.9",
			// Client hint matching the UA brand; some WAFs (bato1.com) reject
			// requests whose Sec-CH-UA is missing or disagrees with it.
			"Sec-CH-UA": secCHUA,
			// Empty Referer by default; only injected when a plugin sets one.
			"Referer": "",
		},
		clients:          make(map[string]tls_client.HttpClient),
		uaOverrides:      make(map[string]string),
		secCHUAExplicit:  "",
		pendingVerify:    make(map[string]verifySeed),
		needsJS:          make(map[string]bool),
		manualCookies:    make(map[string]bool),
		pins:             make(map[string]string),
		hints:            make(map[string][]string),
		pluginCDPTimeout: make(map[string]time.Duration),
		solveChallenge:   solveChallenge,
		stdlibTransport: &nethttp.Transport{
			MaxIdleConnsPerHost: 6,
			IdleConnTimeout:     90 * time.Second,
			TLSHandshakeTimeout: 10 * time.Second,
		},
		SubmitSolve: func(f func()) { go f() },
	}
}

// SetDefaultHeader overrides a default header applied to every request.
func (p *Proxy) SetDefaultHeader(key, value string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.defaultHeaders[key] = value
}

// SetPluginCDPTimeout records a plugin's declared invoke budget (PLUGIN
// timeout key) so browser solves for that plugin use it instead of the
// global cdp_solve_timeout. A zero duration clears the override.
func (p *Proxy) SetPluginCDPTimeout(pluginID string, d time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if d <= 0 {
		delete(p.pluginCDPTimeout, pluginID)
		return
	}
	p.pluginCDPTimeout[pluginID] = d
}

// pluginCDPDeadline returns the effective CDP solve budget for a plugin:
// the plugin's own timeout when declared, else the global cdp value.
func (p *Proxy) pluginCDPDeadline(pluginID string) time.Duration {
	p.mu.Lock()
	defer p.mu.Unlock()
	if d, ok := p.pluginCDPTimeout[pluginID]; ok && d > 0 {
		return d
	}
	return p.cdp.Timeout
}

// SetSecCHUA configures the Sec-CH-UA client hint. An empty value switches
// back to deriving the hint from the effective User-Agent (and dropping it
// for non-Chromium UAs); a non-empty value pins it for every request.
func (p *Proxy) SetSecCHUA(hint string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.secCHUAExplicit = hint
}

// secCHUAHint resolves the Sec-CH-UA to send: the configured value when one
// is pinned, otherwise the Chromium brand/version of ua ("" when ua is not
// Chromium-family, in which case the header must be omitted).
// Callers must hold p.mu.
func (p *Proxy) secCHUAHint(ua string) string {
	if p.secCHUAExplicit != "" {
		return p.secCHUAExplicit
	}
	return secCHUAFor(ua)
}

// ConfigureCDP sets the browser-engine settings used to solve anti-bot
// challenges. Pass CDPConfig{} (or Engine "off") to disable the fallback.
func (p *Proxy) ConfigureCDP(cfg CDPConfig) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cdp = cfg
}

// SetNeedsJS records a plugin's needs_js hint, which routes that plugin's
// requests through the browser engine instead of the fast path when an engine
// is configured.
func (p *Proxy) SetNeedsJS(pluginID string, needsJS bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.needsJS[pluginID] = needsJS
}

// needsJSHint reports whether pluginID declared needs_js.
func (p *Proxy) needsJSHint(pluginID string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.needsJS[pluginID]
}

// SetManualCookies records a plugin's manual_cookies hint: challenges for that
// plugin are never solved by the engine — they fail immediately so the
// human-verify wizard re-opens for a cookie paste.
func (p *Proxy) SetManualCookies(pluginID string, manual bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.manualCookies[pluginID] = manual
}

// manualCookiesHint reports whether pluginID declared manual_cookies.
func (p *Proxy) manualCookiesHint(pluginID string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.manualCookies[pluginID]
}

// CDPConfig returns a copy of the current CDP settings.
func (p *Proxy) CDPConfig() CDPConfig {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.cdp
}

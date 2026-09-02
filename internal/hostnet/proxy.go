package hostnet

import (
	"maps"
	"sync"
	"time"

	http "github.com/bogdanfinn/fhttp"
	tls_client "github.com/bogdanfinn/tls-client"
)

// CDPConfig carries the host-level browser-engine settings for anti-bot
// challenge solving. Engine is "off", "lightpanda", "obscura", or "chrome";
// the zero value (Engine == "") is treated as "off".
type CDPConfig struct {
	Engine  string
	Path    string
	Timeout time.Duration
}

// enabled reports whether a browser engine is configured for challenge solving.
func (c CDPConfig) enabled() bool {
	return c.Engine != "" && c.Engine != "off"
}

// Proxy is a sandboxed HTTP client that enforces standard header injection and
// per-plugin cookie persistence. Plugins must not open sockets directly; all
// network access flows through here.
type Proxy struct {
	mu             sync.Mutex
	defaultHeaders map[string]string
	clients        map[string]tls_client.HttpClient // keyed by plugin id
	uaOverrides    map[string]string                // per-plugin User-Agent override
	pendingVerify  map[string]verifySeed            // cookie jar seeds awaiting client creation
	needsJS        map[string]bool                  // per-plugin needs_js hint
	cdp            CDPConfig

	// solveChallenge is swappable for tests; nil means the real chromedp solver.
	solveChallenge func(cfg CDPConfig, url string) ([]*http.Cookie, string, error)
}

// defaultUA is a browser-like User-Agent so requests are less likely to be
// blocked by anti-bot measures.
const defaultUA = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0 Safari/537.36"

// defaultHeaderOrder fixes the order in which default headers are applied so
// the derived HeaderOrderKey is deterministic regardless of map iteration.
var defaultHeaderOrder = []string{"User-Agent", "Accept-Language", "Referer"}

// NewProxy initializes a Proxy with browser-like default headers and an empty
// per-plugin client map. Each client is created lazily on first use.
func NewProxy() *Proxy {
	return &Proxy{
		defaultHeaders: map[string]string{
			"User-Agent":      defaultUA,
			"Accept-Language": "en-US,en;q=0.9",
			// Empty Referer by default; only injected when a plugin sets one.
			"Referer": "",
		},
		clients:        make(map[string]tls_client.HttpClient),
		uaOverrides:    make(map[string]string),
		pendingVerify:  make(map[string]verifySeed),
		needsJS:        make(map[string]bool),
		solveChallenge: solveChallenge,
	}
}

// DefaultHeaders returns a copy of the default header set applied to every
// request.
func (p *Proxy) DefaultHeaders() map[string]string {
	out := make(map[string]string, len(p.defaultHeaders))
	maps.Copy(out, p.defaultHeaders)
	return out
}

// SetDefaultHeader overrides a default header applied to every request.
func (p *Proxy) SetDefaultHeader(key, value string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.defaultHeaders[key] = value
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

// cdpConfig returns a copy of the current CDP settings.
func (p *Proxy) cdpConfig() CDPConfig {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.cdp
}

package hostnet

import (
	"context"
	"fmt"
	"goisekai/internal/logger"
	nethttp "net/http"
	"net/url"
	"strings"
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
	mu              sync.Mutex
	defaultHeaders  map[string]string
	clients         map[string]tls_client.HttpClient // keyed by pluginID+"\x00"+profile
	uaOverrides     map[string]string                // per-plugin User-Agent override
	secCHUAExplicit string                           // config-set Sec-CH-UA hint ("" = derive from UA)
	pendingVerify   map[string]verifySeed            // cookie jar seeds awaiting client creation
	needsJS         map[string]bool                  // per-plugin needs_js hint
	pins            map[string]string                // pluginID -> pinned profile ("stdlib" = use doRequestStd)
	hints           map[string][]string              // pluginID -> ordered profile ladder from metadata
	persistPin      func(pluginID, profile string)   // persists pin to DB; nil-safe
	cdp             CDPConfig

	// solveChallenge is swappable for tests; nil means the real chromedp solver.
	solveChallenge func(cfg CDPConfig, url string) ([]*http.Cookie, string, error)
	// stdlibTransport is shared by all stdlib requests for connection pooling.
	stdlibTransport *nethttp.Transport
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
		clients:         make(map[string]tls_client.HttpClient),
		uaOverrides:     make(map[string]string),
		secCHUAExplicit: "",
		pendingVerify:   make(map[string]verifySeed),
		needsJS:         make(map[string]bool),
		pins:            make(map[string]string),
		hints:           make(map[string][]string),
		solveChallenge:  solveChallenge,
		stdlibTransport: &nethttp.Transport{
			MaxIdleConnsPerHost: 6,
			IdleConnTimeout:     90 * time.Second,
			TLSHandshakeTimeout: 10 * time.Second,
		},
	}
}

// SetDefaultHeader overrides a default header applied to every request.
func (p *Proxy) SetDefaultHeader(key, value string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.defaultHeaders[key] = value
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

// CDPConfig returns a copy of the current CDP settings.
func (p *Proxy) CDPConfig() CDPConfig {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.cdp
}

// CDPCookie holds a cookie harvested by the CDP engine.
type CDPCookie struct {
	Name     string `json:"name"`
	Value    string `json:"value"`
	Domain   string `json:"domain"`
	Path     string `json:"path"`
	Secure   bool   `json:"secure"`
	HTTPOnly bool   `json:"httpOnly"`
}

// CDPCookies returns cookies from all per-plugin jars matching the domain.
func (p *Proxy) CDPCookies(domain string) []CDPCookie {
	p.mu.Lock()
	defer p.mu.Unlock()
	var out []CDPCookie
	for _, cli := range p.clients {
		cookies := cli.GetCookies(&url.URL{Scheme: "https", Host: domain})
		for _, c := range cookies {
			if c.Domain == domain || strings.HasSuffix(c.Domain, "."+domain) {
				out = append(out, CDPCookie{
					Name: c.Name, Value: c.Value, Domain: c.Domain,
					Path: c.Path, Secure: c.Secure, HTTPOnly: c.HttpOnly,
				})
			}
		}
	}
	return out
}

// TestCDP runs the configured CDP solver against the given URL and returns the
// harvested cookies plus browser User-Agent. Intended for sandbox debugging.
func (p *Proxy) TestCDP(cfg CDPConfig, targetURL string) ([]*http.Cookie, string, error) {
	p.mu.Lock()
	solver := p.solveChallenge
	p.mu.Unlock()
	if solver == nil {
		return nil, "", fmt.Errorf("CDP solver not configured")
	}
	return solver(cfg, targetURL)
}

// Preconnect opens a HEAD request to the given host to warm the connection pool.
// It is fire-and-forget: failures are logged at debug level and silently ignored.
func (p *Proxy) Preconnect(host string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := nethttp.NewRequestWithContext(ctx, nethttp.MethodHead, "https://"+host, nil)
	if err != nil {
		logger.Debug("preconnect build request", "host", host, "error", err)
		return
	}
	req.Header.Set("User-Agent", defaultUA)

	resp, err := p.stdlibTransport.RoundTrip(req)
	if err != nil {
		logger.Debug("preconnect failed", "host", host, "error", err)
		return
	}
	_ = resp.Body.Close()
	if resp.StatusCode >= 400 {
		logger.Debug("preconnect non-2xx", "host", host, "status", resp.StatusCode)
		logger.Debug("transport request completed", "host", host, "status", resp.StatusCode)
		return
	}
	logger.Info("preconnected", "host", host, "status", resp.StatusCode)
}

// GetTransportStats returns current transport stats (idle connections count).
// Returns 0 if stats not available (stdlib Transport has no public stats API).
func (p *Proxy) GetTransportStats() (idleConns int) {
	// stdlib Transport doesn't expose idle connection count publicly
	// We log at debug level on each preconnect instead
	return 0
}

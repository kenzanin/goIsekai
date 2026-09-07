package hostnet

import (
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	http "github.com/bogdanfinn/fhttp"
	tls_client "github.com/bogdanfinn/tls-client"

	"goisekai/pkg/types"
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
	clients        map[string]tls_client.HttpClient // keyed by pluginID+"\x00"+profile
	uaOverrides    map[string]string                // per-plugin User-Agent override
	pendingVerify  map[string]verifySeed            // cookie jar seeds awaiting client creation
	needsJS        map[string]bool                  // per-plugin needs_js hint
	pins           map[string]string                // pluginID -> pinned profile ("stdlib" = use doRequestStd)
	hints          map[string][]string              // pluginID -> ordered profile ladder from metadata
	persistPin     func(pluginID, profile string)   // persists pin to DB; nil-safe
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
		pins:           make(map[string]string),
		hints:          make(map[string][]string),
		solveChallenge: solveChallenge,
	}
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

// SetHTTPProfiles stores a plugin-declared ordered profile ladder. Only
// recognized profile names (or "stdlib") are kept. An empty (or all-filtered)
// list clears any previously stored hints so a manifest that drops
// http_profiles resets the plugin to auto-mode on reload.
func (p *Proxy) SetHTTPProfiles(pluginID string, names []string) {
	var filtered []string
	seen := map[string]bool{}
	for _, n := range names {
		if seen[n] || !isKnownProfile(n) {
			continue
		}
		seen[n] = true
		filtered = append(filtered, n)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(filtered) == 0 {
		delete(p.hints, pluginID)
		return
	}
	p.hints[pluginID] = filtered
}

// SetPinnedProfiles bulk-loads persisted pins at startup.
func (p *Proxy) SetPinnedProfiles(m map[string]string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for id, prof := range m {
		if isKnownProfile(prof) {
			p.pins[id] = prof
		}
	}
}

// SetPersistPin registers a callback invoked whenever a plugin's pin changes.
func (p *Proxy) SetPersistPin(f func(string, string)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.persistPin = f
}

// pin returns the currently pinned profile name for pluginID, or "".
func (p *Proxy) pin(pluginID string) string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.pins[pluginID]
}

// setPin sets the in-memory pin and persists it (if the callback is set).
func (p *Proxy) setPin(pluginID, profile string) {
	p.mu.Lock()
	p.pins[pluginID] = profile
	cb := p.persistPin
	p.mu.Unlock()
	if cb != nil {
		cb(pluginID, profile)
	}
}

// clearPin removes the in-memory pin and persists the empty string.
func (p *Proxy) clearPin(pluginID string) {
	p.setPin(pluginID, "")
}

// ClearPin is the exported wrapper for clearPin, used by the action handler.
func (p *Proxy) ClearPin(pluginID string) {
	p.clearPin(pluginID)
}

// PinnedProfile returns the plugin's pinned profile name, or "" when unpinned.
func (p *Proxy) PinnedProfile(pluginID string) string {
	return p.pin(pluginID)
}

// TestProfile runs a single GET against url using an explicit TLS profile (or
// "stdlib"), returning the resulting HTTP status. It does NOT mutate the
// plugin's pin; used by the plugins-page "Test profile" button.
func (p *Proxy) TestProfile(pluginID, profileName, url string) (int, error) {
	req := types.HTTPRequest{Method: "GET", URL: url}
	if profileName == stdlibProfileName {
		resp, err := p.doRequestStd(pluginID, req)
		if err != nil {
			return 0, err
		}
		return resp.Status, nil
	}
	resp, err := p.doRequestProfile(pluginID, profileName, req)
	if err != nil {
		return 0, err
	}
	return resp.Status, nil
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

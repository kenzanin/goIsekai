package hostnet

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	http "github.com/bogdanfinn/fhttp"
)

// CDPConfig carries the host-level browser-engine settings for anti-bot
// challenge solving. Engine is "off", "lightpanda", "obscura", or "chrome";
// the zero value (Engine == "") is treated as "off".
type CDPConfig struct {
	Engine  string
	Path    string
	Timeout time.Duration
	// Paths holds a per-engine endpoint for the fallback daemons, keyed by
	// engine name ("obscura", "lightpanda"). Without it a fallback engine
	// would inherit Path — the chrome binary — and could never connect,
	// because those engines require a ws:// CDP endpoint.
	Paths map[string]string
}

// pathFor returns the endpoint to use for engine: its own Paths entry when
// present, else the configured Path if engine is the configured one, else "".
func (c CDPConfig) pathFor(engine string) string {
	if p := c.Paths[engine]; p != "" {
		return p
	}
	if engine == c.Engine {
		return c.Path
	}
	return ""
}

// usable reports whether engine has an endpoint of the right shape: daemon
// engines need a ws:// URL, chrome needs a binary path. A fallback engine
// without a usable endpoint is dropped from the chain instead of burning an
// attempt on a connection that cannot succeed.
func (c CDPConfig) usable(engine string) bool {
	p := c.pathFor(engine)
	if p == "" {
		return false
	}
	if engine == "chrome" {
		return true
	}
	return strings.HasPrefix(p, "ws://") || strings.HasPrefix(p, "wss://")
}

// enabled reports whether a browser engine is configured for challenge solving.
func (c CDPConfig) enabled() bool {
	return c.Engine != "" && c.Engine != "off"
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

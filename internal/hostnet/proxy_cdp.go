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

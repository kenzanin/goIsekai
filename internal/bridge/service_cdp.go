package bridge

import (
	"fmt"

	"goisekai/internal/hostnet"
)

// CDPStatus returns the current CDP engine configuration.
func (s *AppService) CDPStatus() hostnet.CDPConfig {
	return s.proxy.CDPConfig()
}

// CDPCookies returns cookies from all per-plugin jars matching the domain.
func (s *AppService) CDPCookies(domain string) []hostnet.CDPCookie {
	return s.proxy.CDPCookies(domain)
}

// TestCDP launches the configured CDP engine against the given URL, waits for
// the challenge to clear, and returns the harvested cookies and User-Agent.
func (s *AppService) TestCDP(targetURL string) ([]hostnet.CDPCookie, string, error) {
	cfg := s.proxy.CDPConfig()
	if cfg.Engine == "" || cfg.Engine == "off" {
		return nil, "", fmt.Errorf("CDP engine is not configured")
	}
	cookies, ua, err := s.proxy.TestCDP(cfg, targetURL)
	if err != nil {
		return nil, "", err
	}
	var out []hostnet.CDPCookie
	for _, c := range cookies {
		out = append(out, hostnet.CDPCookie{
			Name: c.Name, Value: c.Value, Domain: c.Domain,
			Path: c.Path, Secure: c.Secure, HTTPOnly: c.HttpOnly,
		})
	}
	return out, ua, nil
}

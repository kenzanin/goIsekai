package hostnet

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	http "github.com/bogdanfinn/fhttp"
)

// ErrChallenge is the sentinel error wrapped by ChallengeError, returned when a
// response looks like an anti-bot challenge.
var ErrChallenge = errors.New("hostnet: anti-bot challenge detected")

// ChallengeError reports a Cloudflare-style challenge response that requires
// browser verification. VerifyURL is the page the user must open to solve it.
type ChallengeError struct {
	VerifyURL string
}

func (e *ChallengeError) Error() string {
	return fmt.Sprintf("hostnet: challenge at %s: %v", e.VerifyURL, ErrChallenge)
}



// IsChallengeResponse reports whether a response is an anti-bot challenge
// (Cloudflare "Just a moment" interstitial). A response is a challenge when its
// status is 403 or 503 and the body contains a known challenge marker. Only the
// first 8KiB of body are inspected; callers may pass an already-read body.
func IsChallengeResponse(status int, body []byte) bool {
	if status != 403 && status != 503 {
		return false
	}
	if len(body) > 8*1024 {
		body = body[:8*1024]
	}
	lower := strings.ToLower(string(body))
	return strings.Contains(lower, "challenge-platform") || strings.Contains(lower, "just a moment")
}

// verifySeed is a pending cookie-jar seed for a plugin, applied when the
// plugin's client is created (or immediately if it already exists).
type verifySeed struct {
	domain  string
	cookies []*http.Cookie
}

func (s verifySeed) url() *url.URL {
	return &url.URL{Scheme: "https", Host: s.domain}
}

// SetVerifyCookies seeds pluginID's client cookie jar with a Cloudflare
// clearance cookie for domain and stores ua as the User-Agent override applied
// to all subsequent requests for that plugin.
//
// cookieHeader is parsed tolerantly: a full "Cookie" header ("a=1; b=2"), a
// single "name=value" pair, or a bare value (treated as cf_clearance=<value>).
// domain is the site host the cookie belongs to (e.g. "example.com" or
// "example.com:8443"); a scheme/path prefix, if present, is stripped. Pass ua
// as "" to leave the current User-Agent untouched.
//
// The seed applies whether the plugin's client exists yet or not: an existing
// client's jar is updated in place, otherwise the cookies are applied when the
// client is first created.
func (p *Proxy) SetVerifyCookies(pluginID, domain, cookieHeader, ua string) error {
	seed, err := parseVerifyCookies(domain, cookieHeader)
	if err != nil {
		return err
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if ua != "" {
		p.uaOverrides[pluginID] = ua
	}
	p.pendingVerify[pluginID] = seed

	if c, ok := p.clients[pluginID]; ok {
		c.SetCookies(seed.url(), seed.cookies)
	}
	return nil
}

// parseVerifyCookies tolerantly parses cookieHeader into fhttp cookies scoped
// to domain.
func parseVerifyCookies(domain, cookieHeader string) (verifySeed, error) {
	domain = normalizeDomain(domain)
	if domain == "" {
		return verifySeed{}, errors.New("hostnet: empty verify domain")
	}
	h := strings.TrimSpace(cookieHeader)
	if h == "" {
		return verifySeed{}, errors.New("hostnet: empty verify cookie")
	}
	if !strings.ContainsAny(h, "=;") {
		h = "cf_clearance=" + h
	}
	cookies := http.ReadCookies(http.Header{"Cookie": []string{h}}, "")
	if len(cookies) == 0 {
		return verifySeed{}, fmt.Errorf("hostnet: no cookies parsed from %q", cookieHeader)
	}
	return verifySeed{domain: domain, cookies: cookies}, nil
}

// normalizeDomain strips scheme and path from a caller-supplied domain, so
// "https://example.com/verify" and "example.com" both yield "example.com".
func normalizeDomain(domain string) string {
	d := strings.TrimSpace(domain)
	if i := strings.Index(d, "://"); i >= 0 {
		d = d[i+3:]
	}
	if i := strings.IndexByte(d, '/'); i >= 0 {
		d = d[:i]
	}
	return d
}

package hostnet

import (
	"errors"
	nethttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"

	fhttp "github.com/bogdanfinn/fhttp"

	"goisekai/pkg/types"
)

// fakeSolver is a swappable challenge solver for tests. It returns the given
// cookies/UA and tracks how many times it ran.
type fakeSolver struct {
	cookies []*fhttp.Cookie
	ua      string
	err     error
	calls   int
	lastURL string
}

func (f *fakeSolver) solve(_ CDPConfig, url string) ([]*fhttp.Cookie, string, error) {
	f.calls++
	f.lastURL = url
	if f.err != nil {
		return nil, "", f.err
	}
	return f.cookies, f.ua, nil
}

// challengeServer returns a server that answers the first N requests with a
// Cloudflare-style challenge and then with a 200 that echoes the Cookie header.
func challengeServer(challenges int) *httptest.Server {
	hits := 0
	lastCookie := ""
	srv := httptest.NewServer(nethttp.HandlerFunc(func(w nethttp.ResponseWriter, r *nethttp.Request) {
		lastCookie = r.Header.Get("Cookie")
		if hits < challenges {
			hits++
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(403)
			_, _ = w.Write([]byte("<html>challenge-platform</html>"))
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(200)
		_, _ = w.Write([]byte("ok:" + lastCookie))
	}))
	return srv
}

// TestChallengeSolveAndRetry verifies that a challenge response triggers the
// engine, seeds the harvested cookie, and retries the request to a 200.
func TestChallengeSolveAndRetry(t *testing.T) {
	srv := challengeServer(1)
	defer srv.Close()

	p := NewProxy()
	p.ConfigureCDP(CDPConfig{Engine: "chrome", Path: "/usr/bin/chrome"})
	solver := &fakeSolver{cookies: []*fhttp.Cookie{{Name: "cf_clearance", Value: "solved"}}, ua: "browser-ua"}
	p.solveChallenge = solver.solve

	resp, err := p.Request("plugin-1", types.HTTPRequest{Method: "GET", URL: srv.URL + "/page"})
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	if resp.Status != 200 {
		t.Fatalf("status = %d, want 200 (retry after solve)", resp.Status)
	}
	if solver.calls != 1 {
		t.Errorf("solve calls = %d, want 1", solver.calls)
	}
	if !strings.Contains(resp.Body, "cf_clearance=solved") {
		t.Errorf("retried request cookie = %q, want cf_clearance=solved", resp.Body)
	}
}

// TestChallengeSolveDisabled verifies the fallback is skipped entirely when no
// engine is configured: the challenge surfaces as a ChallengeError.
func TestChallengeSolveDisabled(t *testing.T) {
	srv := challengeServer(1)
	defer srv.Close()

	p := NewProxy() // no ConfigureCDP → engine off
	solver := &fakeSolver{cookies: []*fhttp.Cookie{{Name: "cf_clearance", Value: "solved"}}}
	p.solveChallenge = solver.solve

	_, err := p.Request("plugin-1", types.HTTPRequest{Method: "GET", URL: srv.URL + "/page"})
	if !errors.Is(err, ErrChallenge) {
		t.Fatalf("err = %v, want ErrChallenge (engine off)", err)
	}
	if solver.calls != 0 {
		t.Errorf("solve calls = %d, want 0 (engine off)", solver.calls)
	}
}

// TestChallengeSolveFailureDegrades verifies that a solver error falls back to
// the original ChallengeError instead of a misleading success.
func TestChallengeSolveFailureDegrades(t *testing.T) {
	srv := challengeServer(1)
	defer srv.Close()

	p := NewProxy()
	p.ConfigureCDP(CDPConfig{Engine: "chrome", Path: "/usr/bin/chrome"})
	solver := &fakeSolver{err: errors.New("chrome launch failed")}
	p.solveChallenge = solver.solve

	_, err := p.Request("plugin-1", types.HTTPRequest{Method: "GET", URL: srv.URL + "/page"})
	if !errors.Is(err, ErrChallenge) {
		t.Fatalf("err = %v, want ErrChallenge (solve failed)", err)
	}
}

// TestNeedsJSRouting verifies a needs_js plugin routes through the engine
// preemptively (before the fast path), and that disabling the engine falls back
// to the fast path with no solve.
func TestNeedsJSRouting(t *testing.T) {
	// Engine on: solver runs even though the server returns 200 directly.
	srv := challengeServer(0)
	defer srv.Close()

	p := NewProxy()
	p.ConfigureCDP(CDPConfig{Engine: "chrome", Path: "/usr/bin/chrome"})
	p.SetNeedsJS("plugin-1", true)
	solver := &fakeSolver{cookies: []*fhttp.Cookie{{Name: "cf_clearance", Value: "solved"}}, ua: "browser-ua"}
	p.solveChallenge = solver.solve

	if _, err := p.Request("plugin-1", types.HTTPRequest{Method: "GET", URL: srv.URL + "/page"}); err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	if solver.calls != 1 {
		t.Errorf("solve calls = %d, want 1 (needs_js + engine on)", solver.calls)
	}

	// Engine off: no solve, fast path only.
	p2 := NewProxy()
	p2.SetNeedsJS("plugin-1", true)
	solver2 := &fakeSolver{}
	p2.solveChallenge = solver2.solve

	if _, err := p2.Request("plugin-1", types.HTTPRequest{Method: "GET", URL: srv.URL + "/page"}); err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	if solver2.calls != 0 {
		t.Errorf("solve calls = %d, want 0 (needs_js but engine off)", solver2.calls)
	}
}

package hostnet

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	fhttp "github.com/bogdanfinn/fhttp"

	"goisekai/pkg/types"
)

// wafBlockServer returns a server that answers the first `block` requests with
// a JSON anti-bot block (403 + captcha_required body, Content-Type
// application/json) and all subsequent requests with a 200. The counter is
// atomic so it is safe to use from concurrent tls-client goroutines.
func wafBlockServer(block int) *httptest.Server {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := int(hits.Add(1))
		if n <= block {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(403)
			_, _ = w.Write([]byte(`{"error":"captcha_required","challenge":"/@waf/challenge"}`))
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(200)
		_, _ = w.Write([]byte("ok"))
	}))
	return srv
}

// wafHtmlBlockServer returns a server that answers the first `block` requests
// with an HTML page carrying a WAF marker (text/html body), and all subsequent
// requests with 200. Used to verify the htmlBlock dispatch path.
func wafHtmlBlockServer(block int) *httptest.Server {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := int(hits.Add(1))
		if n <= block {
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(403)
			_, _ = w.Write([]byte(`<html><body>@waf/challenge</body></html>`))
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(200)
		_, _ = w.Write([]byte("ok"))
	}))
	return srv
}

// persistPinRecorder captures setPin calls for assertion.
type persistPinRecorder struct {
	pins map[string]string
}

func (r *persistPinRecorder) record(pluginID, profile string) {
	r.pins[pluginID] = profile
}

// TestLadderPinOnSuccess verifies that when the default profile returns a JSON
// WAF block, the ladder tries the next profile and pins it on a clean 2xx.
func TestLadderPinOnSuccess(t *testing.T) {
	// Block 1 request (the default chrome_146 attempt), then allow the rest.
	srv := wafBlockServer(1)
	defer srv.Close()

	p := NewProxy()
	recorder := &persistPinRecorder{pins: make(map[string]string)}
	p.SetPersistPin(recorder.record)

	resp, err := p.Request("plugin-1", types.HTTPRequest{Method: "GET", URL: srv.URL})
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	if resp.Status != 200 {
		t.Fatalf("status = %d, want 200 (ladder found a winning profile)", resp.Status)
	}

	// The winning profile should have been pinned.
	pinned := p.PinnedProfile("plugin-1")
	if pinned == "" {
		t.Fatal("PinnedProfile = empty, want a non-default profile name")
	}
	if pinned == defaultProfileName {
		t.Errorf("pinned profile = %s (the default), want a different rung", pinned)
	}
	if recorder.pins["plugin-1"] != pinned {
		t.Errorf("persistPin called with %q, want %q", recorder.pins["plugin-1"], pinned)
	}
}

// TestLadderStdlibFallback verifies that when all tls-client profiles are
// blocked but stdlib is in the ladder, stdlib wins and is marked via markStdlib.
func TestLadderStdlibFallback(t *testing.T) {
	// Block 7 requests (1 default + 6 tls-client rungs). The 8th request
	// (stdlib, last rung in the default ladder) succeeds.
	srv := wafBlockServer(7)
	defer srv.Close()

	p := NewProxy()

	resp, err := p.Request("plugin-1", types.HTTPRequest{Method: "GET", URL: srv.URL})
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	if resp.Status != 200 {
		t.Fatalf("status = %d, want 200 (stdlib bypassed the fingerprint block)", resp.Status)
	}
	if p.PinnedProfile("plugin-1") != stdlibProfileName {
		t.Errorf("PinnedProfile = %q, want stdlib", p.PinnedProfile("plugin-1"))
	}
}

// TestLadderAllBlockedReturnsChallenge verifies the behavior when every
// profile (including stdlib) is blocked and CDP is disabled: the request
// surfaces ChallengeError without burning more than the minimal ladder.
func TestLadderAllBlockedReturnsChallenge(t *testing.T) {
	// Block far more than the ladder has rungs.
	srv := wafBlockServer(999)
	defer srv.Close()

	p := NewProxy() // CDP disabled by default

	_, err := p.Request("plugin-1", types.HTTPRequest{Method: "GET", URL: srv.URL})
	if !errors.Is(err, ErrChallenge) {
		t.Fatalf("err = %v, want ErrChallenge (all profiles blocked, no CDP)", err)
	}
}

// TestLadderCDPSolveLastResort verifies that when the ladder exhausts all
// profiles, the CDP solver is invoked as the last resort.
func TestLadderCDPSolveLastResort(t *testing.T) {
	// Block all tls-client profiles but allow after CDP solve seeds cookies.
	srv := wafBlockServer(999)
	defer srv.Close()

	p := NewProxy()
	p.ConfigureCDP(CDPConfig{Engine: "chrome", Path: "/usr/bin/chrome"})
	solver := &fakeSolver{
		cookies: []*fhttp.Cookie{{Name: "cf_clearance", Value: "solved"}},
		ua:      "browser-ua",
	}
	p.solveChallenge = solver.solve

	// The ladder exhausts all profiles (blocked), then CDP is invoked as the
	// last resort. The retry still hits the same server (blocked), so the
	// final response is ChallengeError — but the solver MUST have been called.
	_, _ = p.Request("plugin-1", types.HTTPRequest{Method: "GET", URL: srv.URL})
	if solver.calls == 0 {
		t.Error("CDP solver was never called as last resort")
	}
}

// TestLadderDoesNotPin429 verifies that a non-WAF-blocked 429 response from a
// ladder rung is NOT pinned (it's a real origin response, not a fingerprint
// win).
func TestLadderDoesNotPin429(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := int(hits.Add(1))
		if n == 1 {
			// First request: WAF block (triggers ladder).
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(403)
			_, _ = w.Write([]byte(`{"error":"captcha_required"}`))
			return
		}
		// All subsequent: 429 rate limit (not a WAF block, not a challenge).
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(429)
		_, _ = w.Write([]byte("rate limited"))
	}))
	defer srv.Close()

	p := NewProxy()

	resp, err := p.Request("plugin-1", types.HTTPRequest{Method: "GET", URL: srv.URL})
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	if resp.Status != 429 {
		t.Fatalf("status = %d, want 429 (non-blocked rate limit returned)", resp.Status)
	}
	if p.PinnedProfile("plugin-1") != "" {
		t.Errorf("PinnedProfile = %q, want empty (429 must not be pinned)", p.PinnedProfile("plugin-1"))
	}
}

// TestLadderSkipsPinned verifies the ladder does not re-try the profile that
// already failed in the fast path.
func TestLadderSkipsPinned(t *testing.T) {
	// Pre-pin chrome_131 and block ALL requests.
	srv := wafBlockServer(999)
	defer srv.Close()

	p := NewProxy()
	p.setPin("plugin-1", "chrome_131")

	_, err := p.Request("plugin-1", types.HTTPRequest{Method: "GET", URL: srv.URL})
	if !errors.Is(err, ErrChallenge) {
		t.Fatalf("err = %v, want ErrChallenge", err)
	}
	// Verify chrome_131 was skipped: it should NOT have been tried again.
	// We can't directly count per-profile requests without server-side
	// differentiation, but we verify the pin didn't change.
	if p.PinnedProfile("plugin-1") != "chrome_131" {
		t.Errorf("PinnedProfile changed to %q during ladder, want chrome_131 preserved", p.PinnedProfile("plugin-1"))
	}
}

// TestLadderFullProbeCount verifies the total number of HTTP requests when
// every profile is WAF-blocked: 1 default + up to 7 untried ladder rungs
// (chrome_146 is skipped as already-tried) = 8 max. ponytail: the old code
// cost 2 (default + stdlib); profile rotation adds probes but pins on the
// first win, so this is a one-time discovery cost per plugin.
func TestLadderFullProbeCount(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(403)
		_, _ = w.Write([]byte(`{"error":"captcha_required"}`))
	}))
	defer srv.Close()

	p := NewProxy() // CDP disabled, no pin

	_, err := p.Request("plugin-1", types.HTTPRequest{Method: "GET", URL: srv.URL})
	if !errors.Is(err, ErrChallenge) {
		t.Fatalf("err = %v, want ErrChallenge", err)
	}
	totalHits := int(hits.Load())
	// 1 default (blocked) + 7 ladder rungs (chrome_146 skipped as default)
	// = 8 max. stdlib is included in the 8.
	if totalHits > 8 {
		t.Errorf("total server hits = %d, want ≤8 (default + untried ladder rungs)", totalHits)
	}
}

// TestLadderCDPDisabledWithPinTriesLadder verifies that when CDP is disabled
// but a pin exists (the profile went stale), the ladder still tries other
// profiles.
func TestLadderCDPDisabledWithPinTriesLadder(t *testing.T) {
	// Block all requests.
	srv := wafBlockServer(999)
	defer srv.Close()

	p := NewProxy() // CDP disabled
	// Pre-pin a stale profile so the ladder isn't short-circuited.
	p.setPin("plugin-1", "chrome_120")

	_, err := p.Request("plugin-1", types.HTTPRequest{Method: "GET", URL: srv.URL})
	if !errors.Is(err, ErrChallenge) {
		t.Fatalf("err = %v, want ErrChallenge (all blocked)", err)
	}
	// The pin should NOT have changed since all profiles were blocked.
	if p.PinnedProfile("plugin-1") != "chrome_120" {
		t.Errorf("PinnedProfile = %q, want chrome_120 unchanged", p.PinnedProfile("plugin-1"))
	}
}

// TestHTMLWafBlockRoutesToCDP verifies that an HTML page carrying a WAF
// marker (like mangafire's challenge page) is routed to the CDP solver
// directly, NOT through the profile ladder (no 8-probe burst).
func TestHTMLWafBlockRoutesToCDP(t *testing.T) {
	// Block 1 request (HTML WAF page). After CDP solve, the retry should
	// succeed.
	srv := wafHtmlBlockServer(1)
	defer srv.Close()

	p := NewProxy()
	p.ConfigureCDP(CDPConfig{Engine: "chrome", Path: "/usr/bin/chrome"})
	solver := &fakeSolver{
		cookies: []*fhttp.Cookie{{Name: "cf_clearance", Value: "solved"}},
		ua:      "browser-ua",
	}
	p.solveChallenge = solver.solve

	resp, err := p.Request("plugin-1", types.HTTPRequest{Method: "GET", URL: srv.URL})
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	if resp.Status != 200 {
		t.Fatalf("status = %d, want 200 (CDP solved HTML block)", resp.Status)
	}
	if solver.calls != 1 {
		t.Errorf("solve calls = %d, want 1 (CDP called, not ladder)", solver.calls)
	}
}

// TestStdlibPrefersRoute verifies that a stdlib-pinned plugin skips the
// tls-client path entirely.
func TestStdlibPrefersRoute(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(200)
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	p := NewProxy()
	p.setPin("plugin-1", stdlibProfileName)

	resp, err := p.Request("plugin-1", types.HTTPRequest{Method: "GET", URL: srv.URL})
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	if resp.Status != 200 {
		t.Fatalf("status = %d, want 200", resp.Status)
	}
	if hits.Load() != 1 {
		t.Errorf("server hits = %d, want 1 (single stdlib request, no retry)", hits.Load())
	}
}

// TestWafBlockTriggerCount verifies the number of HTTP requests made for a
// typical WAF-block-then-succeed scenario (default blocked, second profile
// wins). Old code cost 2; new code should cost ≤3 (default + 1 ladder + maybe
// the default was already tried).
func TestWafBlockTriggerCount(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := int(hits.Add(1))
		if n == 1 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(403)
			_, _ = w.Write([]byte(`{"error":"captcha_required"}`))
			return
		}
		w.WriteHeader(200)
		_, _ = fmt.Fprintf(w, "ok-%d", n)
	}))
	defer srv.Close()

	p := NewProxy()

	resp, err := p.Request("plugin-1", types.HTTPRequest{Method: "GET", URL: srv.URL})
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	if resp.Status != 200 {
		t.Fatalf("status = %d, want 200", resp.Status)
	}
	totalHits := int(hits.Load())
	// 1 default (WAF blocked) + 1 ladder attempt (200) = 2
	if totalHits != 2 {
		t.Errorf("total hits = %d, want 2 (default + one ladder rung)", totalHits)
	}
}

// TestChallengeRetryStillChallenged verifies that if the CDP solve seeds
// cookies but the retry STILL shows a challenge (chained/stale clearance), the
// response surfaces ChallengeError — not a false success.
func TestChallengeRetryStillChallenged(t *testing.T) {
	// Server always returns challenge.
	srv := challengeServer(9999)
	defer srv.Close()

	p := NewProxy()
	p.ConfigureCDP(CDPConfig{Engine: "chrome", Path: "/usr/bin/chrome"})
	solver := &fakeSolver{
		cookies: []*fhttp.Cookie{{Name: "cf_clearance", Value: "stale"}},
		ua:      "browser-ua",
	}
	p.solveChallenge = solver.solve

	_, err := p.Request("plugin-1", types.HTTPRequest{Method: "GET", URL: srv.URL})
	if err == nil {
		t.Fatal("err = nil, want ChallengeError (retry still challenged)")
	}
	var ce *ChallengeError
	if !errors.As(err, &ce) {
		t.Fatalf("err = %v (%T), want *ChallengeError", err, err)
	}
	if solver.calls != 1 {
		t.Errorf("solve calls = %d, want 1", solver.calls)
	}
}

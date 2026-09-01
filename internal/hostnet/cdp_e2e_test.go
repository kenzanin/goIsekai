package hostnet

import (
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"sync/atomic"
	"testing"
	"time"
)

// lightpandaURL returns the CDP endpoint to test against, or skips the test if
// nothing is reachable. Override with LIGHTPANDA_URL; defaults to the local
// `lightpanda serve` default (ws://127.0.0.1:9222).
func lightpandaURL(t *testing.T) string {
	t.Helper()
	u := os.Getenv("LIGHTPANDA_URL")
	if u == "" {
		u = "ws://127.0.0.1:9222"
	}
	host := u
	if parsed, err := url.Parse(u); err == nil && parsed.Host != "" {
		host = parsed.Host
	}
	if !tcpAlive(host) {
		t.Skipf("lightpanda not reachable at %s (run `lightpanda serve`)", u)
	}
	return u
}

// tcpAlive reports whether a TCP connection to addr can be established within
// 1s — a fast guard so chromedp doesn't hang on dial when the server is down.
func tcpAlive(addr string) bool {
	conn, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// mockChallengeSite simulates a Cloudflare managed-challenge flow: without the
// solved cookie it serves 403 + a challenge marker plus a script that solves
// the challenge by fetching /grant (which sets the cookie via Set-Cookie, the
// real cf_clearance path) and reloading; with the cookie it serves a normal
// page. This exercises the real browser engine (JS execution, cookie set,
// reload) deterministically.
type mockChallengeSite struct {
	solved atomic.Value // string
}

func (m *mockChallengeSite) handler(w http.ResponseWriter, r *http.Request) {
	if c, _ := r.Cookie("cf_clearance"); c != nil && c.Value == m.solved.Load().(string) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html><body>real content</body></html>"))
		return
	}
	// The JS triggers /grant, which sets the cookie via a Set-Cookie header
	// (matching how Cloudflare actually issues cf_clearance), then reloads.
	if r.URL.Path == "/grant" {
		http.SetCookie(w, &http.Cookie{Name: "cf_clearance", Value: m.solved.Load().(string), Path: "/"})
		w.WriteHeader(200)
		return
	}
	// Challenge: solve by fetching /grant (proves the engine executes JS),
	// then reload so the next navigation returns the real page.
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(403)
	_, _ = w.Write([]byte(`<html><body>
<div id="challenge-platform">challenge-platform</div>
<script>
fetch("/grant").then(function(){ location.reload(); });
</script>
</body></html>`))
}

// TestSolveChallengeAgainstLightpanda drives the real solveChallenge against a
// real lightpanda CDP server. It verifies the linchpin assumption of the whole
// cdp-engine feature: chromedp can attach lightpanda, run the page's JS (which
// sets the cookie), wait out the challenge, and harvest the cookie back.
func TestSolveChallengeAgainstLightpanda(t *testing.T) {
	url := lightpandaURL(t)

	site := &mockChallengeSite{}
	site.solved.Store("solved123")
	srv := httptest.NewServer(http.HandlerFunc(site.handler))
	defer srv.Close()

	cookies, ua, err := solveChallenge(CDPConfig{
		Engine:  "lightpanda",
		Path:    url,
		Timeout: 15 * time.Second,
	}, srv.URL)
	if err != nil {
		t.Fatalf("solveChallenge failed: %v", err)
	}

	var got string
	for _, c := range cookies {
		if c.Name == "cf_clearance" {
			got = c.Value
		}
	}
	if got != "solved123" {
		t.Errorf("harvested cf_clearance = %q, want solved123 (all cookies: %+v)", got, cookies)
	}
	if ua == "" {
		t.Errorf("browser User-Agent was empty")
	}
}

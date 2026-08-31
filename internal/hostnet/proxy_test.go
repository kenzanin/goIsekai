package hostnet

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"goisekai/pkg/types"
)

// TestRequestInjectsHeaders verifies default User-Agent is applied and a
// per-page Referer override wins over the empty default Referer.
func TestRequestInjectsHeaders(t *testing.T) {
	var gotUA, gotReferer string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		gotReferer = r.Header.Get("Referer")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	p := NewProxy()
	if _, err := p.Request("plugin-1", types.HTTPRequest{
		Method:  "GET",
		URL:     srv.URL,
		Headers: map[string]string{"Referer": "https://example.com/override"},
	}); err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	if gotUA != defaultUA {
		t.Errorf("User-Agent = %q, want %q", gotUA, defaultUA)
	}
	if gotReferer != "https://example.com/override" {
		t.Errorf("Referer = %q, want override %q", gotReferer, "https://example.com/override")
	}
}

// TestRequestPersistsCookies verifies a cookie set by the server on the first
// request is resent on the second request for the same plugin id.
func TestRequestPersistsCookies(t *testing.T) {
	var cookieSeen string
	setOnce := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookieSeen = r.Header.Get("Cookie")
		if !setOnce {
			http.SetCookie(w, &http.Cookie{Name: "sid", Value: "abc123"})
			setOnce = true
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	p := NewProxy()

	if _, err := p.Request("plugin-1", types.HTTPRequest{Method: "GET", URL: srv.URL + "/set"}); err != nil {
		t.Fatalf("first Request failed: %v", err)
	}
	if cookieSeen != "" {
		t.Errorf("first request Cookie = %q, want empty (jar was empty)", cookieSeen)
	}

	if _, err := p.Request("plugin-1", types.HTTPRequest{Method: "GET", URL: srv.URL + "/get"}); err != nil {
		t.Fatalf("second Request failed: %v", err)
	}
	if !strings.Contains(cookieSeen, "sid=abc123") {
		t.Errorf("second request Cookie = %q, want to contain sid=abc123", cookieSeen)
	}
}

// TestParseVerifyCookies covers the tolerant cookie-header parser: a full
// "Cookie" header, a single name=value pair, and a bare value (cf_clearance).
func TestParseVerifyCookies(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		want   map[string]string
		wantOK bool
	}{
		{"full cookie header", "a=1; b=2", map[string]string{"a": "1", "b": "2"}, true},
		{"single pair", "name=value", map[string]string{"name": "value"}, true},
		{"bare value", "cdef0123", map[string]string{"cf_clearance": "cdef0123"}, true},
		{"empty", "", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seed, err := parseVerifyCookies("example.com", tt.input)
			if tt.wantOK != (err == nil) {
				t.Fatalf("parseVerifyCookies(%q) err = %v, wantOK %v", tt.input, err, tt.wantOK)
			}
			if !tt.wantOK {
				return
			}
			if seed.domain != "example.com" {
				t.Errorf("domain = %q, want example.com", seed.domain)
			}
			got := make(map[string]string, len(seed.cookies))
			for _, c := range seed.cookies {
				got[c.Name] = c.Value
			}
			if len(got) != len(tt.want) {
				t.Fatalf("parsed cookies = %v, want %v", got, tt.want)
			}
			for name, val := range tt.want {
				if got[name] != val {
					t.Errorf("cookie %s = %q, want %q", name, got[name], val)
				}
			}
		})
	}
}

// TestSetVerifyCookiesSeedsJar verifies a seeded cookie survives client
// creation and is sent on subsequent requests to the same host.
func TestSetVerifyCookiesSeedsJar(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte(r.Header.Get("Cookie")))
	}))
	defer srv.Close()

	domain := strings.TrimPrefix(srv.URL, "http://")
	p := NewProxy()

	// Seeded before the client exists: pending seed must apply on creation.
	if err := p.SetVerifyCookies("plugin-1", domain, "cf_clearance=abc123; a=1", "ua-1"); err != nil {
		t.Fatalf("SetVerifyCookies: %v", err)
	}

	for i := 0; i < 2; i++ {
		resp, err := p.Request("plugin-1", types.HTTPRequest{Method: "GET", URL: srv.URL + "/check"})
		if err != nil {
			t.Fatalf("Request %d failed: %v", i, err)
		}
		if !strings.Contains(resp.Body, "cf_clearance=abc123") {
			t.Errorf("request %d Cookie = %q, want to contain cf_clearance=abc123", i, resp.Body)
		}
		if !strings.Contains(resp.Body, "a=1") {
			t.Errorf("request %d Cookie = %q, want to contain a=1", i, resp.Body)
		}
	}

	// Re-seed with an existing client: jar must update in place.
	if err := p.SetVerifyCookies("plugin-1", domain, "cf_clearance=xyz789", ""); err != nil {
		t.Fatalf("SetVerifyCookies (existing client): %v", err)
	}
	resp, err := p.Request("plugin-1", types.HTTPRequest{Method: "GET", URL: srv.URL + "/recheck"})
	if err != nil {
		t.Fatalf("re-seeded Request failed: %v", err)
	}
	if strings.Contains(resp.Body, "abc123") {
		t.Errorf("Cookie after re-seed = %q, want old cf_clearance replaced", resp.Body)
	}
	if !strings.Contains(resp.Body, "cf_clearance=xyz789") {
		t.Errorf("Cookie after re-seed = %q, want to contain cf_clearance=xyz789", resp.Body)
	}
}

// TestSetVerifyCookiesSetsUserAgent verifies the UA override replaces the
// default and per-request User-Agent on every request.
func TestSetVerifyCookiesSetsUserAgent(t *testing.T) {
	var gotUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	domain := strings.TrimPrefix(srv.URL, "http://")
	p := NewProxy()
	if err := p.SetVerifyCookies("plugin-1", domain, "cf_clearance=abc", "override-UA"); err != nil {
		t.Fatalf("SetVerifyCookies: %v", err)
	}

	if _, err := p.Request("plugin-1", types.HTTPRequest{
		Method:  "GET",
		URL:     srv.URL,
		Headers: map[string]string{"User-Agent": "per-request-UA"},
	}); err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	if gotUA != "override-UA" {
		t.Errorf("User-Agent = %q, want override-UA (override beats per-request)", gotUA)
	}

	// Second request without per-request UA: override still applies.
	if _, err := p.Request("plugin-1", types.HTTPRequest{Method: "GET", URL: srv.URL}); err != nil {
		t.Fatalf("second Request failed: %v", err)
	}
	if gotUA != "override-UA" {
		t.Errorf("User-Agent = %q, want override-UA on plain request", gotUA)
	}
}

// TestIsChallengeResponse covers the status/marker matrix.
func TestIsChallengeResponse(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
		want   bool
	}{
		{"403 with challenge-platform", 403, "<html>challenge-platform</html>", true},
		{"403 with Just a moment", 403, "Just a moment...", true},
		{"403 without marker", 403, "<html>normal</html>", false},
		{"200 with marker", 200, "challenge-platform", false},
		{"503 with marker", 503, "challenge-platform", true},
		{"404 without marker", 404, "<html>not found</html>", false},
		{"marker beyond 8KiB ignored", 403, "x" + strings.Repeat(" ", 8*1024) + "challenge-platform", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsChallengeResponse(tt.status, []byte(tt.body)); got != tt.want {
				t.Errorf("IsChallengeResponse(%d, body) = %v, want %v", tt.status, got, tt.want)
			}
		})
	}
}

// TestChallengeError checks the exported challenge error type wraps ErrChallenge.
func TestChallengeError(t *testing.T) {
	err := &ChallengeError{VerifyURL: "https://example.com/cdn-cgi/challenge"}
	if !errors.Is(err, ErrChallenge) {
		t.Errorf("errors.Is(%v, ErrChallenge) = false, want true", err)
	}
	if err.Error() == "" || !strings.Contains(err.Error(), "example.com") {
		t.Errorf("ChallengeError.Error() = %q, want verify URL included", err.Error())
	}
}

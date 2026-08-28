package hostnet

import (
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

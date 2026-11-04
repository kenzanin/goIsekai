package hostnet

// Task 1.6: plugin-declared User-Agent ladder tests. The test server echoes
// the User-Agent it received so assertions read directly from the request.

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"goisekai/pkg/types"
)

// uaEchoServer records every request's User-Agent and answers `block`
// requests with a WAF block before serving 200 "ok".
func uaEchoServer(t *testing.T, block int, uas *[]string, mu *atomic.Int32) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := int(mu.Add(1))
		*uas = append(*uas, r.Header.Get("User-Agent"))
		if n <= block {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(403)
			_, _ = w.Write([]byte(`{"error":"captcha_required","challenge":"/@waf/challenge"}`))
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("ok"))
	}))
}

// first entry on request one
func TestUAFirstEntryOnRequestOne(t *testing.T) {
	var hits atomic.Int32
	var uas []string
	srv := uaEchoServer(t, 0, &uas, &hits)
	defer srv.Close()

	p := NewProxy()
	p.SetUserAgents("p1", []string{"goIsekai/1.0 (personal reader)", "default"})

	resp, err := p.Request("p1", types.HTTPRequest{Method: "GET", URL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	if got := uas[len(uas)-1]; got != "goIsekai/1.0 (personal reader)" {
		t.Fatalf("first request UA = %q, want the first declared entry", got)
	}
	if resp.Status != 200 {
		t.Fatalf("status = %d", resp.Status)
	}
}

// fallback to the second entry on a block, pin reused afterwards
func TestUAFallbackOnBlockThenPinned(t *testing.T) {
	var hits atomic.Int32
	var uas []string
	// First (default-UA) request blocked, second (first declared entry)
	// blocked, third (second entry) succeeds. Then one more request reuses
	// the pin.
	srv := uaEchoServer(t, 2, &uas, &hits)
	defer srv.Close()

	p := NewProxy()
	p.SetUserAgents("p1", []string{"ua-a", "ua-b"})

	resp, err := p.Request("p1", types.HTTPRequest{Method: "GET", URL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	if got := uas[len(uas)-1]; got != "ua-b" {
		t.Fatalf("fallback UA = %q, want ua-b", got)
	}
	if resp.Status != 200 {
		t.Fatalf("status = %d", resp.Status)
	}
	if pin := p.uaPin("p1"); pin != "ua-b" {
		t.Fatalf("uaPin = %q, want ua-b (winning entry remembered)", pin)
	}
	// Subsequent request starts with the pinned entry, no new block needed.
	if _, err := p.Request("p1", types.HTTPRequest{Method: "GET", URL: srv.URL}); err != nil {
		t.Fatal(err)
	}
	if got := uas[len(uas)-1]; got != "ua-b" {
		t.Fatalf("post-pin UA = %q, want ua-b reused", got)
	}
}

// every entry blocked returns the challenge error
func TestUAAllBlockedReturnsChallenge(t *testing.T) {
	srv := uaEchoServer(t, 999, &[]string{}, &atomic.Int32{})
	defer srv.Close()

	p := NewProxy() // CDP disabled by default
	p.SetUserAgents("p1", []string{"ua-a", "ua-b"})

	_, err := p.Request("p1", types.HTTPRequest{Method: "GET", URL: srv.URL})
	if !errors.Is(err, ErrChallenge) {
		t.Fatalf("err = %v, want ErrChallenge", err)
	}
}

// verify-seeded UA wins over the declared list
func TestUAVerifyOverrideWins(t *testing.T) {
	var hits atomic.Int32
	var uas []string
	srv := uaEchoServer(t, 0, &uas, &hits)
	defer srv.Close()

	p := NewProxy()
	p.SetUserAgents("p1", []string{"ua-a"})
	if err := p.SetVerifyCookies("p1", "example.com", "cf_clearance=x", "verify-ua"); err != nil {
		t.Fatalf("SetVerifyCookies: %v", err)
	}

	req := types.HTTPRequest{Method: "GET", URL: srv.URL}
	resp, rerr := p.Request("p1", req)
	if rerr != nil {
		t.Fatal(rerr)
	}
	if got := uas[len(uas)-1]; got != "verify-ua" {
		t.Fatalf("UA = %q, want the verify-seeded value", got)
	}
	if resp.Status != 200 {
		t.Fatalf("status = %d", resp.Status)
	}
}

// per-request header wins over the declared list
func TestUAPerRequestHeaderWins(t *testing.T) {
	var hits atomic.Int32
	var uas []string
	srv := uaEchoServer(t, 0, &uas, &hits)
	defer srv.Close()

	p := NewProxy()
	p.SetUserAgents("p1", []string{"ua-a"})

	req := types.HTTPRequest{
		Method:  "GET",
		URL:     srv.URL,
		Headers: map[string]string{"User-Agent": "explicit-ua"},
	}
	if _, err := p.Request("p1", req); err != nil {
		t.Fatal(err)
	}
	if got := uas[len(uas)-1]; got != "explicit-ua" {
		t.Fatalf("UA = %q, want explicit-ua", got)
	}
}

// no Sec-CH-UA for a non-Chromium declared entry
func TestUANonChromiumOmitsClientHint(t *testing.T) {
	var gotCHUA atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if v, ok := r.Header["Sec-Ch-Ua"]; ok {
			gotCHUA.Store(strings.Join(v, ","))
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	p := NewProxy()
	p.SetUserAgents("p1", []string{"goIsekai/1.0 (personal reader)"})

	if _, err := p.Request("p1", types.HTTPRequest{Method: "GET", URL: srv.URL}); err != nil {
		t.Fatal(err)
	}
	if v := gotCHUA.Load(); v != nil {
		t.Fatalf("Sec-CH-UA = %v, want omitted for a non-Chromium UA", v)
	}
}

// a plugin with no declared list keeps the host default
func TestUANoListKeepsHostDefault(t *testing.T) {
	var hits atomic.Int32
	var uas []string
	srv := uaEchoServer(t, 0, &uas, &hits)
	defer srv.Close()

	p := NewProxy()
	if _, err := p.Request("p1", types.HTTPRequest{Method: "GET", URL: srv.URL}); err != nil {
		t.Fatal(err)
	}
	if got := uas[len(uas)-1]; got != defaultUA {
		t.Fatalf("UA = %q, want host default %q", got, defaultUA)
	}
}

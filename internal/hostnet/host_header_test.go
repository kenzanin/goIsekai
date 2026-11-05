package hostnet

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"goisekai/pkg/types"
)

// TestRequestHostHeaderOverride checks whether a Host header passed by a plugin
// reaches the server on both request paths (tls-client/fhttp and stdlib) —
// the vhost override mangade needs while api.mangade.io's certificate is
// expired.
func TestRequestHostHeaderOverride(t *testing.T) {
	var gotHosts []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHosts = append(gotHosts, r.Host)
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	// fhttp/tls-client path (default profile).
	p := NewProxy()
	if _, err := p.Request("probe-host-fhttp", types.HTTPRequest{
		Method:  "GET",
		URL:     srv.URL,
		Headers: map[string]string{"Host": "api.mangade.io"},
	}); err != nil {
		t.Fatalf("fhttp-path request: %v", err)
	}

	// stdlib path (pinned "stdlib").
	p2 := NewProxy()
	p2.pins["probe-host-stdlib"] = stdlibProfileName
	if _, err := p2.Request("probe-host-stdlib", types.HTTPRequest{
		Method:  "GET",
		URL:     srv.URL,
		Headers: map[string]string{"Host": "api.mangade.io"},
	}); err != nil {
		t.Fatalf("stdlib-path request: %v", err)
	}

	t.Logf("hosts seen: %q", gotHosts)
	if len(gotHosts) != 2 {
		t.Fatalf("saw %d requests, want 2", len(gotHosts))
	}
	for i, h := range gotHosts {
		if h != "api.mangade.io" {
			t.Errorf("path %d: Host = %q, want api.mangade.io", i, h)
		}
	}
}

package hostnet

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"goisekai/pkg/types"
)

// Task 3.1: a cancelled context tears down the in-flight upstream call instead
// of letting it run to the client timeout. The handler here blocks for far
// longer than the test is willing to wait, so returning promptly is only
// possible if cancellation actually reached the socket.
func TestRequestContextCancelsInFlightCall(t *testing.T) {
	released := make(chan struct{})
	var hits atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		select {
		case <-released:
		case <-r.Context().Done():
		case <-time.After(60 * time.Second):
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer func() {
		close(released)
		srv.Close()
	}()

	p := NewProxy()
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	_, err := p.RequestContext(ctx, "plugin-cancel", types.HTTPRequest{
		Method: "GET",
		URL:    srv.URL,
	})
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("RequestContext returned nil error for a cancelled call")
	}
	if elapsed > 10*time.Second {
		t.Fatalf("RequestContext took %v; the cancel did not reach the upstream call", elapsed)
	}
}

// Task 3.1: an already-cancelled context is refused before any socket work.
func TestRequestContextAlreadyCancelled(t *testing.T) {
	var hits atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	p := NewProxy()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := p.RequestContext(ctx, "plugin-dead", types.HTTPRequest{
		Method: "GET",
		URL:    srv.URL,
	}); err == nil {
		t.Fatal("RequestContext with a cancelled context returned no error")
	}
	if n := hits.Load(); n != 0 {
		t.Errorf("server was hit %d times for an already-cancelled context, want 0", n)
	}
}

// Request stays usable for callers with no context of their own (the plugin
// ABI), which is why RequestContext was added alongside Request rather than
// replacing it.
func TestRequestWithoutContextStillWorks(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	p := NewProxy()
	resp, err := p.Request("plugin-plain", types.HTTPRequest{Method: "GET", URL: srv.URL})
	if err != nil {
		t.Fatalf("Request: %v", err)
	}
	if resp.Status != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.Status)
	}
}

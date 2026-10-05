package bridge

import (
	"bytes"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// imageFetchBudget must sit above one request timeout (so a single slow attempt
// is reported honestly) and below two (so a dead origin is not paid for twice).
func TestImageFetchBudgetIsSane(t *testing.T) {
	if imageFetchBudget <= 0 {
		t.Fatal("budget is not positive")
	}
	if imageFetchBudget < 25*time.Second {
		t.Errorf("budget = %s, too tight: one slow attempt would be cut off mid-flight", imageFetchBudget)
	}
	if imageFetchBudget > 60*time.Second {
		t.Errorf("budget = %s, too loose: a dead origin still costs over a minute of spinner", imageFetchBudget)
	}
}

// A fast failure costs nothing to repeat, so it must still be retried. The
// budget only stops attempts that have already spent it.
func TestFastFailureStillRetries(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	t.Cleanup(srv.Close)

	s := newTestServiceWithCache(t)
	if _, err := s.GetImage(t.Context(), "p", srv.URL+"/x.png", nil, "m", "c", PrioLow); err == nil {
		t.Fatal("GetImage succeeded against a 503 server")
	}
	if got := hits.Load(); got != 3 {
		t.Errorf("upstream hits = %d, want 3; a fast 503 must still use the whole ladder", got)
	}
}

// A 200 with a good body must never touch the retry path at all.
func TestSuccessIsNotRetried(t *testing.T) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 4, 4))); err != nil {
		t.Fatal(err)
	}
	payload := buf.Bytes()
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(payload)
	}))
	t.Cleanup(srv.Close)

	s := newTestServiceWithCache(t)
	if _, err := s.GetImage(t.Context(), "p", srv.URL+"/ok.png", nil, "m", "c", PrioLow); err != nil {
		t.Fatalf("GetImage: %v", err)
	}
	if got := hits.Load(); got != 1 {
		t.Errorf("upstream hits = %d, want 1", got)
	}
}

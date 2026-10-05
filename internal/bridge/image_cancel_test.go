package bridge

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"goisekai/internal/workers"
)

// findImageLane returns the image lane's reported state.
func findImageLane(t *testing.T, s *AppService) workers.LaneState {
	t.Helper()
	for _, st := range s.pool.LaneStates() {
		if st.Lane == workers.LaneImage {
			return st
		}
	}
	t.Fatal("image lane missing from LaneStates()")
	return workers.LaneState{}
}

// Task 3.1 / 3.2: a cancelled caller abandons the image fetch. The handler here
// blocks far longer than the test will wait, so returning promptly proves the
// cancellation reached the upstream call rather than only the response.
func TestGetImageAbandonsOnCancelledContext(t *testing.T) {
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
	t.Cleanup(func() {
		close(released)
		srv.Close()
	})

	s := newTestServiceWithCache(t)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	_, err := s.GetImage(ctx, "plugin-cancel", srv.URL+"/img", nil, "", "", PrioLow)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("GetImage returned nil error for a cancelled caller")
	}
	if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("GetImage error = %v, want a context cancellation error", err)
	}
	if elapsed > 10*time.Second {
		t.Fatalf("GetImage took %v; cancellation did not reach the upstream call", elapsed)
	}
}

// Task 3.2: an already-cancelled caller never issues the request at all.
func TestGetImageAlreadyCancelledSkipsUpstream(t *testing.T) {
	var hits atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	s := newTestServiceWithCache(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := s.GetImage(ctx, "plugin-dead", srv.URL+"/img", nil, "", "", PrioLow); err == nil {
		t.Fatal("GetImage with a cancelled context returned no error")
	}
	if n := hits.Load(); n != 0 {
		t.Errorf("upstream was hit %d times for an already-cancelled caller, want 0", n)
	}
}

// Task 3.3: the image lane counts an abandoned fetch as abandoned, not failed.
// A reader that skips chapters should not make the lane look like it is
// erroring.
func TestImageLaneClassifiesAbandonedNotFailed(t *testing.T) {
	released := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-released:
		case <-r.Context().Done():
		case <-time.After(60 * time.Second):
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(func() {
		close(released)
		srv.Close()
	})

	s := newTestServiceWithCache(t)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()
	if _, err := s.GetImage(ctx, "plugin-cancel", srv.URL+"/img", nil, "", "", PrioLow); err == nil {
		t.Fatal("expected the cancelled fetch to fail")
	}

	var st workers.LaneState
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		st = findImageLane(t, s)
		if st.Abandoned > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if st.Abandoned < 1 {
		t.Fatalf("image lane abandoned=%d failed=%d, want at least 1 abandoned", st.Abandoned, st.Failed)
	}
	if st.Failed != 0 {
		t.Errorf("image lane failed=%d, want 0: a cancelled caller is not an upstream failure", st.Failed)
	}
}

// Task 4.4: an abandoned fetch leaves nothing in the caches, so a later real
// request still fetches from upstream.
func TestAbandonedGetImageLeavesNoCacheEntry(t *testing.T) {
	var hits atomic.Int64
	released := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		select {
		case <-released:
		case <-r.Context().Done():
		case <-time.After(60 * time.Second):
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(func() {
		close(released)
		srv.Close()
	})

	s := newTestServiceWithCache(t)
	url := srv.URL + "/img"
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.GetImage(ctx, "plugin-cancel", url, nil, "", "", PrioLow); err == nil {
		t.Fatal("expected the cancelled fetch to fail")
	}
	if _, ok := s.imageCache[url]; ok {
		t.Error("L1 cache holds an entry for an abandoned fetch")
	}
}

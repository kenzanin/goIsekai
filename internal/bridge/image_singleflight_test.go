package bridge

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Regression test for the singleflight race in GetImage.
//
// The broken version stored a throwaway imageCall via LoadOrStore, then built a
// second call and Stored it over the top. A goroutine arriving between those two
// operations got the throwaway's done channel, which no job ever closes, so it
// blocked forever. It passed in isolation (the window is a few instructions) and
// hung the full package suite for 230s under parallel load.
//
// The shape below hits that window by starting many callers at once against a
// server that delays, so the leader is reliably still in flight while the
// followers arrive. Every caller must return; one that does not is the bug.
func TestGetImageSingleflightNeverParksACaller(t *testing.T) {
	var hits atomic.Int32
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 4, 4))); err != nil {
		t.Fatalf("png.Encode: %v", err)
	}
	payload := buf.Bytes()

	// Delay keeps the leader in flight while followers pile in, which is exactly
	// the window the race lived in.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		time.Sleep(5 * time.Millisecond)
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(payload)
	}))
	t.Cleanup(srv.Close)

	s := newTestServiceWithCache(t)

	// A distinct PATH per round keeps every round a genuine cache miss. The
	// disk cache keys on the path component alone (query strings vary on signed
	// CDN URLs), so varying only the query would be served from round 0.
	const rounds = 40
	const callers = 16

	for round := range rounds {
		url := srv.URL + "/img/round/" + strconv.Itoa(round)
		before := hits.Load()

		var wg sync.WaitGroup
		var returned atomic.Int32
		results := make([][]byte, callers)
		errs := make([]error, callers)
		for i := range callers {
			wg.Go(func() {
				results[i], errs[i] = s.GetImage(context.Background(), "p", url, nil, "m", "c", PrioLow)
				returned.Add(1)
			})
		}

		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
		case <-time.After(30 * time.Second):
			// Do not let the bug hang the suite: fail with the round that wedged.
			t.Fatalf("round %d: only %d of %d callers returned; the singleflight "+
				"left a caller waiting on a done channel nobody closes",
				round, returned.Load(), callers)
		}

		for i := range callers {
			if errs[i] != nil {
				t.Fatalf("round %d caller %d: %v", round, i, errs[i])
			}
			if len(results[i]) == 0 {
				t.Fatalf("round %d caller %d returned no bytes", round, i)
			}
		}
		if got := hits.Load() - before; got != 1 {
			t.Errorf("round %d: upstream hits = %d, want 1 (callers must share one fetch)", round, got)
		}
	}
}

// Every call that enters the singleflight map must be owned by the goroutine that
// created it, so the job's cleanup removes it. A leftover entry means a stored
// call was never the one a job ran, which is the structural half of the bug.
func TestGetImageSingleflightLeavesNoOrphanEntry(t *testing.T) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 4, 4))); err != nil {
		t.Fatalf("png.Encode: %v", err)
	}
	payload := buf.Bytes()

	release := make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }

	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		<-release
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(payload)
	}))
	t.Cleanup(func() {
		unblock()
		srv.Close()
	})

	s := newTestServiceWithCache(t)
	url := srv.URL + "/img"

	const callers = 24
	var wg sync.WaitGroup
	for range callers {
		wg.Go(func() {
			_, _ = s.GetImage(context.Background(), "p", url, nil, "m", "c", PrioLow)
		})
	}

	// While the single fetch is in flight the map must hold exactly one entry:
	// the leader's own call. A second entry would mean two call objects exist
	// for one URL, which is the shape that orphaned waiters.
	waitFor(t, func() bool { return s.imageFlightCount() == 1 }, "the leader to register its call")
	if got := s.imageFlightCount(); got != 1 {
		t.Fatalf("in-flight singleflight entries = %d, want 1", got)
	}

	// Let the fetch finish so every job runs its cleanup.
	unblock()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("callers never returned")
	}

	// Cleanup deletes the entry before closing done, so once the last caller is
	// unblocked the map must be empty.
	waitFor(t, func() bool { return s.imageFlightCount() == 0 },
		"every caller to finish and the map to drain")
	if got := s.imageFlightCount(); got != 0 {
		t.Errorf("%d orphan entries left in the singleflight map", got)
	}
	if got := hits.Load(); got != 1 {
		t.Errorf("upstream hits = %d, want 1", got)
	}
}

// A caller that arrives while the leader is fetching must receive the leader's
// bytes rather than starting its own fetch. This is the behaviour the race would
// have degraded into a duplicate request had the orphan not parked first.
func TestGetImageFollowerReusesLeaderResult(t *testing.T) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 4, 4))); err != nil {
		t.Fatalf("png.Encode: %v", err)
	}
	payload := buf.Bytes()

	release := make(chan struct{})
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		<-release
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(payload)
	}))
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	t.Cleanup(func() {
		unblock()
		srv.Close()
	})

	s := newTestServiceWithCache(t)
	url := srv.URL + "/img"

	leaderDone := make(chan struct{})
	go func() {
		defer close(leaderDone)
		if _, err := s.GetImage(context.Background(), "p", url, nil, "m", "c", PrioLow); err != nil {
			t.Errorf("leader: %v", err)
		}
	}()
	waitFor(t, func() bool { return s.imageFlightCount() == 1 }, "the leader to register its call")

	followerDone := make(chan []byte, 1)
	go func() {
		data, err := s.GetImage(context.Background(), "p", url, nil, "m", "c", PrioLow)
		if err != nil {
			t.Errorf("follower: %v", err)
		}
		followerDone <- data
	}()

	// Release the fetch; both callers must observe the same single request.
	unblock()
	<-leaderDone

	select {
	case data := <-followerDone:
		if len(data) == 0 {
			t.Error("follower returned no bytes")
		}
	case <-time.After(30 * time.Second):
		t.Fatal("follower never returned; it is waiting on an unshared done channel")
	}

	if got := hits.Load(); got != 1 {
		t.Errorf("upstream hits = %d, want 1; the follower started its own fetch", got)
	}
}

// imageFlightCount reports how many URLs are currently in the singleflight map.
func (s *AppService) imageFlightCount() int {
	n := 0
	s.imageFlight.Range(func(_, _ any) bool {
		n++
		return true
	})
	return n
}

// waitFor polls cond until it holds or the test gives up. Used to synchronise on
// the singleflight map without sleeping for a fixed time.
func waitFor(t *testing.T, cond func() bool, what string) {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

package bridge

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"goisekai/internal/hostnet"
)

// task 3.1: runOnFetch is the seam every fetch-shaped path rides (migration
// candidate search, enrichment fetch, cover refetch). A slow plugin must
// serialize its own jobs while other plugins keep their own slot.
func TestRunOnFetchPerPluginFairness(t *testing.T) {
	s := newTestService(t)
	defer s.Shutdown()

	var slowInFlight, maxSlow atomic.Int32
	slowEntered := make(chan struct{}, 2)
	release := make(chan struct{})

	var wg sync.WaitGroup
	for range 2 {
		wg.Go(func() {
			_ = s.runOnFetch(context.Background(), "slow-plugin", func(context.Context) error {
				if n := slowInFlight.Add(1); n > maxSlow.Load() {
					maxSlow.Store(n)
				}
				slowEntered <- struct{}{}
				<-release
				slowInFlight.Add(-1)
				return nil
			})
		})
	}

	// Hold the slow plugin's slot first, so the probe below is meaningful.
	select {
	case <-slowEntered:
	case <-time.After(3 * time.Second):
		t.Fatal("slow plugin job never started")
	}

	otherDone := make(chan struct{})
	go func() {
		_ = s.runOnFetch(context.Background(), "other-plugin", func(context.Context) error {
			close(otherDone)
			return nil
		})
	}()

	// The other plugin must finish while the slow slot is still held.
	select {
	case <-otherDone:
	case <-time.After(3 * time.Second):
		t.Fatal("other plugin was blocked behind the slow one")
	}

	close(release)
	wg.Wait()

	if maxSlow.Load() != 1 {
		t.Fatalf("slow plugin ran %d jobs at once, want 1", maxSlow.Load())
	}
}

// An anti-bot challenge needs a human to open VerifyURL, so re-invoking the
// plugin reproduces it and only hammers the site. The fetch lane defaults to 3
// attempts, so without workers.Permanent the job would run three times for the
// same wall. The typed error must still reach the caller: the views around this
// seam re-open the human-verify wizard by matching *hostnet.ChallengeError.
func TestRunOnFetchDoesNotRetryAChallenge(t *testing.T) {
	s := newTestService(t)
	defer s.Shutdown()

	var attempts atomic.Int32
	err := s.runOnFetch(context.Background(), "plugin-x", func(context.Context) error {
		attempts.Add(1)
		return &hostnet.ChallengeError{VerifyURL: "https://example.test/verify"}
	})

	if n := attempts.Load(); n != 1 {
		t.Fatalf("a challenge must not be retried, got %d attempts", n)
	}
	if _, ok := errors.AsType[*hostnet.ChallengeError](err); !ok {
		t.Fatalf("the challenge type must survive the retry wrapper, got %#v", err)
	}
}

// Everything else stays retryable: a bare error is the transient case the retry
// loop exists for, so it must still run MaxAttempts times.
func TestRunOnFetchRetriesATransientError(t *testing.T) {
	s := newTestService(t)
	defer s.Shutdown()

	var attempts atomic.Int32
	_ = s.runOnFetch(context.Background(), "plugin-x", func(context.Context) error {
		attempts.Add(1)
		return errors.New("temporary upstream blip")
	})

	if n := attempts.Load(); n != 3 {
		t.Fatalf("a transient error should retry to MaxAttempts (3), got %d attempts", n)
	}
}

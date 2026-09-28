package bridge

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
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
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = s.runOnFetch(context.Background(), "slow-plugin", func(context.Context) error {
				if n := slowInFlight.Add(1); n > maxSlow.Load() {
					maxSlow.Store(n)
				}
				slowEntered <- struct{}{}
				<-release
				slowInFlight.Add(-1)
				return nil
			})
		}()
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


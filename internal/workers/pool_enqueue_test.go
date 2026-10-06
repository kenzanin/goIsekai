package workers

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// A refused enqueue must leave nothing behind. The job is published to
// p.registry and p.dedupe before it is pushed onto a lane, so a refused push
// has to undo both - otherwise the next enqueue of the same DedupeKey would join
// a future for a job that never ran.
//
// Saturation is deterministic: one worker, one queue slot, the worker occupied
// by a job that blocks, so the next enqueue cannot be placed.
func TestRefusedEnqueueLeavesNoRegistryOrDedupeEntry(t *testing.T) {
	release := make(chan struct{})
	started := make(chan struct{})

	p := New(Config{
		FetchSize:                1,
		FetchQueue:               1,
		BackgroundEnqueueTimeout: 40 * time.Millisecond,
	})
	t.Cleanup(p.Shutdown)

	if _, err := p.Enqueue(context.Background(), &Job{
		Lane:      LaneFetch,
		DedupeKey: "occupy-worker",
		Run: func(context.Context) error {
			close(started)
			<-release
			return nil
		},
	}); err != nil {
		t.Fatalf("enqueue the blocking job: %v", err)
	}
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("the fetch worker never picked up the blocking job")
	}
	if _, err := p.Enqueue(context.Background(), &Job{
		Lane:      LaneFetch,
		DedupeKey: "fill-queue",
		Run:       func(context.Context) error { return nil },
	}); err != nil {
		t.Fatalf("enqueue the queue-filling job: %v", err)
	}

	refused := &Job{
		Lane:      LaneFetch,
		DedupeKey: "refused",
		Run:       func(context.Context) error { return nil },
	}
	if fut, err := p.Enqueue(context.Background(), refused); !errors.Is(err, ErrBusy) {
		close(release)
		t.Fatalf("Enqueue = (%v, %v), want ErrBusy", fut, err)
	}

	p.regMu.Lock()
	_, inRegistry := p.registry[refused.ID]
	dedupeKey, inDedupe := p.dedupe[refused.DedupeKey]
	p.regMu.Unlock()
	if inRegistry {
		t.Error("the refused job is still in the registry")
	}
	if inDedupe {
		t.Errorf("the refused job is still in the dedupe map under %q", dedupeKey)
	}

	close(release)
}

// failQueued completes the future exactly once. A double call must not panic on
// the second close, which is what makes it safe to wire into every failure path.
func TestFailQueuedIsIdempotent(t *testing.T) {
	// jobHandle.fut is built by Enqueue before any failure path runs, so mirror
	// that here rather than leaving the future nil.
	h := &jobHandle{fut: &Future{}}
	h.fut.done = make(chan struct{})
	h.fut.info = h

	want := errors.New("dropped")
	h.failQueued(want)
	h.failQueued(errors.New("second call"))

	select {
	case <-h.fut.done:
	default:
		t.Fatal("failQueued left the future open")
	}
	if !errors.Is(h.fut.result, want) {
		t.Errorf("future result = %v, want %v", h.fut.result, want)
	}
}

// A follower that is handed a future while the leader is still being placed must
// observe the leader's failure, not park. This drives the real dedupe path: the
// follower races the leader's publish window and then awaits.
func TestDedupeFollowerSeesTheLeadersFailure(t *testing.T) {
	p := New(Config{InteractiveSize: 1, InteractiveQueue: 1})
	t.Cleanup(p.Shutdown)

	var seen int
	var mu sync.Mutex
	job := func() *Job {
		return &Job{
			Lane:      LaneInteractive,
			DedupeKey: "shared",
			// Without this the retry wrapper reruns Run on its own error, which
			// would make the run count say nothing about dedupe.
			MaxAttempts: 1,
			Run: func(context.Context) error {
				mu.Lock()
				seen++
				mu.Unlock()
				return errors.New("upstream said no")
			},
		}
	}

	first, err := p.Enqueue(context.Background(), job())
	if err != nil {
		t.Fatalf("first enqueue: %v", err)
	}

	// A second enqueue of the same key must join the first job's future rather
	// than start a duplicate.
	second, err := p.Enqueue(context.Background(), job())
	if err != nil {
		t.Fatalf("follower enqueue: %v", err)
	}
	if second != first {
		t.Error("the follower received a different future instead of the leader's")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := second.Await(ctx); err == nil {
		t.Error("Await returned nil, want the leader's error")
	}

	mu.Lock()
	defer mu.Unlock()
	if seen != 1 {
		t.Errorf("the job ran %d times, want 1 (callers must share one job)", seen)
	}
}

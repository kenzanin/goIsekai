package workers

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// task 1.1: typed jobs run and deliver results through a ctx-aware future.
func TestFutureDeliversResult(t *testing.T) {
	p := New(Config{InteractiveSize: 2, InteractiveQueue: 4})
	defer p.Shutdown()

	var ran atomic.Bool
	fut, err := p.Enqueue(context.Background(), &Job{
		Lane: LaneInteractive,
		Run: func(ctx context.Context) error {
			ran.Store(true)
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := fut.Await(ctx); err != nil {
		t.Fatalf("await: %v", err)
	}
	if !ran.Load() {
		t.Fatal("job did not run")
	}
}

// task 1.1: Await honors ctx — abandoned wait cancels the queued job.
func TestAwaitCtxCancelAbandonsJob(t *testing.T) {
	p := New(Config{MaintenanceSize: 1, MaintenanceQueue: 4})
	defer p.Shutdown()

	// Occupy the single maintenance worker.
	block := make(chan struct{})
	_, err := p.Enqueue(context.Background(), &Job{
		Lane: LaneMaintenance,
		Run: func(ctx context.Context) error {
			<-block
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	var ran atomic.Bool
	fut, err := p.Enqueue(context.Background(), &Job{
		Lane: LaneMaintenance,
		Run: func(ctx context.Context) error {
			ran.Store(true)
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if err := fut.Await(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("want DeadlineExceeded, got %v", err)
	}
	close(block)
	time.Sleep(100 * time.Millisecond)
	if ran.Load() {
		t.Fatal("abandoned queued job must not run")
	}
}

// task 1.1: registry tracks status transitions.
func TestRegistryLifecycle(t *testing.T) {
	p := New(Config{MaintenanceSize: 1, MaintenanceQueue: 2})
	defer p.Shutdown()

	started := make(chan struct{})
	release := make(chan struct{})
	fut, err := p.Enqueue(context.Background(), &Job{
		ID:   "job-1",
		Lane: LaneMaintenance,
		Run: func(ctx context.Context) error {
			close(started)
			<-release
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	<-started
	info, ok := p.Status("job-1")
	if !ok || info.Status != StatusRunning {
		t.Fatalf("want running, got %+v ok=%v", info, ok)
	}
	close(release)
	if err := fut.Await(context.Background()); err != nil {
		t.Fatal(err)
	}
	// Registry entry is removed after completion (short-lived jobs).
	if _, ok := p.Status("job-1"); ok {
		t.Fatal("completed job should be deregistered")
	}
}

// task 1.2: transient errors retry with a budget, then dead.
func TestTransientRetryThenDead(t *testing.T) {
	p := New(Config{MaintenanceSize: 1, MaintenanceQueue: 2})
	defer p.Shutdown()

	var attempts atomic.Int32
	fut, err := p.Enqueue(context.Background(), &Job{
		Lane:        LaneMaintenance,
		MaxAttempts: 3,
		Run: func(ctx context.Context) error {
			attempts.Add(1)
			return errors.New("boom")
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	err = fut.Await(context.Background())
	if err == nil || err.Error() != "boom" {
		t.Fatalf("want final boom, got %v", err)
	}
	if n := attempts.Load(); n != 3 {
		t.Fatalf("want 3 attempts, got %d", n)
	}
}

// task 1.2: permanent errors do not retry.
func TestPermanentNoRetry(t *testing.T) {
	p := New(Config{MaintenanceSize: 1, MaintenanceQueue: 2})
	defer p.Shutdown()

	var attempts atomic.Int32
	fut, err := p.Enqueue(context.Background(), &Job{
		Lane:        LaneMaintenance,
		MaxAttempts: 3,
		Run: func(ctx context.Context) error {
			attempts.Add(1)
			return Permanent(errors.New("404"))
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := fut.Await(context.Background()); err == nil {
		t.Fatal("want error")
	}
	if n := attempts.Load(); n != 1 {
		t.Fatalf("permanent error must not retry, got %d attempts", n)
	}
}

// task 1.2: transient failure that succeeds on retry ends Done.
func TestRetryRecovers(t *testing.T) {
	p := New(Config{MaintenanceSize: 1, MaintenanceQueue: 2})
	defer p.Shutdown()

	var attempts atomic.Int32
	fut, err := p.Enqueue(context.Background(), &Job{
		Lane:        LaneMaintenance,
		MaxAttempts: 3,
		Run: func(ctx context.Context) error {
			if attempts.Add(1) < 2 {
				return errors.New("flaky")
			}
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := fut.Await(context.Background()); err != nil {
		t.Fatalf("want success after retry, got %v", err)
	}
	if n := attempts.Load(); n != 2 {
		t.Fatalf("want 2 attempts, got %d", n)
	}
}

// task 1.3: interactive enqueue blocks to ctx deadline then ErrEnqueueTimeout.
func TestInteractiveBackpressure(t *testing.T) {
	p := New(Config{InteractiveSize: 1, InteractiveQueue: 1})
	defer p.Shutdown()

	block := make(chan struct{})
	defer close(block)
	_, err := p.Enqueue(context.Background(), &Job{
		Lane: LaneInteractive,
		Run: func(ctx context.Context) error {
			<-block
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	// Fill the queue slot.
	_, err = p.Enqueue(context.Background(), &Job{
		Lane: LaneInteractive,
		Run:  func(ctx context.Context) error { return nil },
	})
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()
	_, err = p.Enqueue(ctx, &Job{
		Lane: LaneInteractive,
		Run:  func(ctx context.Context) error { return nil },
	})
	if !errors.Is(err, ErrEnqueueTimeout) {
		t.Fatalf("want ErrEnqueueTimeout, got %v", err)
	}
}

// task 1.3: background lanes return ErrBusy within the short timeout —
// never unbounded goroutines or unbounded blocking.
func TestBackgroundBackpressureBusy(t *testing.T) {
	p := New(Config{
		ImageSize:                1,
		ImageQueue:               1,
		BackgroundEnqueueTimeout: 40 * time.Millisecond,
	})
	defer p.Shutdown()

	block := make(chan struct{})
	defer close(block)
	_, err := p.Enqueue(context.Background(), &Job{
		Lane: LaneImage, Priority: PriorityLow,
		Run: func(ctx context.Context) error { <-block; return nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = p.Enqueue(context.Background(), &Job{
		Lane: LaneImage, Priority: PriorityLow,
		Run: func(ctx context.Context) error { return nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = p.Enqueue(context.Background(), &Job{
		Lane: LaneImage, Priority: PriorityLow,
		Run: func(ctx context.Context) error { return nil },
	})
	if !errors.Is(err, ErrBusy) {
		t.Fatalf("want ErrBusy, got %v", err)
	}
}

// task 1.1: image lane drains high priority before low.
func TestImagePriorityOrder(t *testing.T) {
	p := New(Config{ImageSize: 1, ImageQueue: 8})
	defer p.Shutdown()

	block := make(chan struct{})
	// Occupy the single image worker.
	_, err := p.Enqueue(context.Background(), &Job{
		Lane: LaneImage, Priority: PriorityLow,
		Run: func(ctx context.Context) error { <-block; return nil },
	})
	if err != nil {
		t.Fatal(err)
	}

	var mu sync.Mutex
	var order []string
	add := func(s string) {
		mu.Lock()
		order = append(order, s)
		mu.Unlock()
	}
	// Enqueue low then high while the worker is blocked: high must win.
	for _, j := range []struct {
		name string
		prio Priority
	}{{"low-1", PriorityLow}, {"high-1", PriorityHigh}, {"low-2", PriorityLow}, {"high-2", PriorityHigh}} {
		name := j.name
		_, err := p.Enqueue(context.Background(), &Job{
			Lane: LaneImage, Priority: j.prio,
			Run: func(ctx context.Context) error { add(name); return nil },
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	close(block)
	// Wait for all four to drain.
	deadline := time.After(2 * time.Second)
	for {
		mu.Lock()
		n := len(order)
		mu.Unlock()
		if n == 4 {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("timeout, order=%v", order)
		case <-time.After(10 * time.Millisecond):
		}
	}
	mu.Lock()
	defer mu.Unlock()
	// First two completions must be the high ones (high queue drained first).
	highs := 0
	for _, name := range order[:2] {
		if name == "high-1" || name == "high-2" {
			highs++
		}
	}
	if highs != 2 {
		t.Fatalf("high priority must drain first, order=%v", order)
	}
}

// task 1.1: fetch lane enforces one in-flight job per PluginKey.
func TestFetchPluginFairness(t *testing.T) {
	p := New(Config{FetchSize: 2, FetchQueue: 8})
	defer p.Shutdown()

	var inflight atomic.Int32
	var maxSeen atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		_, err := p.Enqueue(context.Background(), &Job{
			Lane:      LaneFetch,
			PluginKey: "same-plugin",
			Run: func(ctx context.Context) error {
				defer wg.Done()
				n := inflight.Add(1)
				for {
					m := maxSeen.Load()
					if n <= m || maxSeen.CompareAndSwap(m, n) {
						break
					}
				}
				time.Sleep(30 * time.Millisecond)
				inflight.Add(-1)
				return nil
			},
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	wg.Wait()
	if maxSeen.Load() != 1 {
		t.Fatalf("same PluginKey must serialize, max in-flight = %d", maxSeen.Load())
	}
}

// task 1.4: defaults apply when no config present.
func TestConfigDefaults(t *testing.T) {
	c := Config{}.withDefaults()
	if c.InteractiveSize != 4 || c.FetchSize != 2 || c.ImageSize != 8 || c.MaintenanceSize != 1 {
		t.Fatalf("unexpected defaults: %+v", c)
	}
	if c.InteractiveQueue != 32 || c.ImageQueue != 64 {
		t.Fatalf("unexpected queue defaults: %+v", c)
	}
}

// task 1.4: ResizeLane grows worker counts.
func TestResizeLaneGrows(t *testing.T) {
	p := New(Config{FetchSize: 1, FetchQueue: 4})
	defer p.Shutdown()
	p.ResizeLane(LaneFetch, 3)
	if got := p.nFetch.Load(); got != 3 {
		t.Fatalf("want 3 fetch workers, got %d", got)
	}
	// shrink is a no-op
	p.ResizeLane(LaneFetch, 1)
	if got := p.nFetch.Load(); got != 3 {
		t.Fatalf("shrink must be no-op, got %d", got)
	}
}

// Shutdown is idempotent and rejects new work.
func TestShutdownRejects(t *testing.T) {
	p := New(Config{MaintenanceSize: 1, MaintenanceQueue: 1})
	p.Shutdown()
	p.Shutdown()
	_, err := p.Enqueue(context.Background(), &Job{
		Lane: LaneMaintenance,
		Run:  func(ctx context.Context) error { return nil },
	})
	if !errors.Is(err, ErrClosed) {
		t.Fatalf("want ErrClosed, got %v", err)
	}
}

// task 3.2: a second enqueue with the same DedupeKey returns the in-flight
// job instead of starting a duplicate (design spec scenario).
func TestDedupeReturnsInflightJob(t *testing.T) {
	p := New(Config{MaintenanceSize: 1, MaintenanceQueue: 8})
	defer p.Shutdown()

	started := make(chan struct{})
	release := make(chan struct{})
	runs := 0
	first, err := p.Enqueue(context.Background(), &Job{
		Lane:      LaneMaintenance,
		DedupeKey: "sync:library",
		Run: func(ctx context.Context) error {
			runs++
			close(started)
			<-release
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	<-started

	second, err := p.Enqueue(context.Background(), &Job{
		Lane:      LaneMaintenance,
		DedupeKey: "sync:library",
		Run: func(ctx context.Context) error {
			runs++
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if first.GetID() != second.GetID() {
		t.Fatalf("dedupe returned a different job: %s vs %s", first.GetID(), second.GetID())
	}

	close(release)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := first.Await(ctx); err != nil {
		t.Fatalf("await: %v", err)
	}
	if runs != 1 {
		t.Fatalf("job body ran %d times, want 1", runs)
	}
}

// task 3.2: once the job is done the key is free again — a later enqueue
// starts a fresh job rather than resolving to a dead future.
func TestDedupeKeyFreedAfterCompletion(t *testing.T) {
	p := New(Config{MaintenanceSize: 1, MaintenanceQueue: 8})
	defer p.Shutdown()

	key := "sync:manga:p1:m1"
	first, err := p.Enqueue(context.Background(), &Job{
		Lane:      LaneMaintenance,
		DedupeKey: key,
		Run:       func(ctx context.Context) error { return nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := first.Await(ctx); err != nil {
		t.Fatalf("await: %v", err)
	}

	var ran atomic.Bool
	second, err := p.Enqueue(context.Background(), &Job{
		Lane:      LaneMaintenance,
		DedupeKey: key,
		Run: func(ctx context.Context) error {
			ran.Store(true)
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if first.GetID() == second.GetID() {
		t.Fatal("completed job still owned the dedupe key")
	}
	if err := second.Await(ctx); err != nil {
		t.Fatalf("await second: %v", err)
	}
	if !ran.Load() {
		t.Fatal("second job did not run")
	}
}

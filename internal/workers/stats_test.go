package workers

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// findLane returns the reported state for one lane.
func findLane(t *testing.T, p *Pool, lane Lane) LaneState {
	t.Helper()
	for _, st := range p.LaneStates() {
		if st.Lane == lane {
			return st
		}
	}
	t.Fatalf("lane %s missing from LaneStates()", lane)
	return LaneState{}
}

// Task 2.2: all four lanes are reported with zero values when idle, whether or
// not they have ever been used.
func TestLaneStatesReportsAllLanesIdle(t *testing.T) {
	p := New(Config{})
	defer p.Shutdown()

	states := p.LaneStates()
	if len(states) != 4 {
		t.Fatalf("LaneStates() returned %d lanes, want 4", len(states))
	}
	for _, st := range states {
		if st.Waiting != 0 || st.Running != 0 {
			t.Errorf("lane %s: waiting=%d running=%d on a fresh pool, want 0/0", st.Lane, st.Waiting, st.Running)
		}
		if st.Capacity <= 0 {
			t.Errorf("lane %s: capacity=%d, want the configured worker count", st.Lane, st.Capacity)
		}
		if st.Completed != 0 || st.Failed != 0 || st.Abandoned != 0 {
			t.Errorf("lane %s: counters %d/%d/%d on a fresh pool, want 0/0/0",
				st.Lane, st.Completed, st.Failed, st.Abandoned)
		}
	}
}

// Task 2.2: queued work is visible, and a lane whose workers are all busy
// reports running == capacity with at least one waiting.
func TestLaneStatesShowsQueuedAndRunning(t *testing.T) {
	// One worker, so exactly one job can run and the rest must queue.
	p := New(Config{MaintenanceSize: 1, MaintenanceQueue: 8})
	defer p.Shutdown()

	release := make(chan struct{})
	var started sync.WaitGroup
	started.Add(1)
	firstDone := make(chan struct{})

	if _, err := p.Enqueue(context.Background(), &Job{
		Lane: LaneMaintenance,
		Run: func(ctx context.Context) error {
			started.Done()
			<-release
			return nil
		},
	}); err != nil {
		t.Fatal(err)
	}
	started.Wait()

	// Fill the queue behind the busy worker.
	const queued = 3
	for i := 0; i < queued; i++ {
		if _, err := p.Enqueue(context.Background(), &Job{
			Lane: LaneMaintenance,
			Run:  func(ctx context.Context) error { <-firstDone; return nil },
		}); err != nil {
			t.Fatal(err)
		}
	}

	st := findLane(t, p, LaneMaintenance)
	if st.Running != 1 {
		t.Errorf("running=%d, want 1 (capacity is 1)", st.Running)
	}
	if st.Waiting < 1 {
		t.Errorf("waiting=%d, want at least 1 while the sole worker is blocked", st.Waiting)
	}
	if st.Capacity != 1 {
		t.Errorf("capacity=%d, want 1", st.Capacity)
	}

	close(release)
	close(firstDone)
}

// Task 2.3 + 2.4: completed jobs record a duration, and summarising reports the
// sample count, a median and a p99 inside the observed range.
func TestLaneDurationSummary(t *testing.T) {
	p := New(Config{})
	defer p.Shutdown()

	// Idle lane: explicitly absent, not a zero duration.
	st := findLane(t, p, LaneInteractive)
	if st.Durations.HasData {
		t.Errorf("fresh lane reports HasData=%v, want false", st.Durations.HasData)
	}
	if st.Durations.Median != 0 || st.Durations.P99 != 0 {
		t.Errorf("fresh lane reports median=%v p99=%v, want 0/0 with HasData=false",
			st.Durations.Median, st.Durations.P99)
	}

	// One slow job and three fast ones, so median and p99 must differ and both
	// must land inside [fast, slow].
	for i := 0; i < 3; i++ {
		if _, err := p.Enqueue(context.Background(), &Job{
			Lane: LaneInteractive,
			Run: func(ctx context.Context) error {
				time.Sleep(2 * time.Millisecond)
				return nil
			},
		}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := p.Enqueue(context.Background(), &Job{
		Lane: LaneInteractive,
		Run: func(ctx context.Context) error {
			time.Sleep(120 * time.Millisecond)
			return nil
		},
	}); err != nil {
		t.Fatal(err)
	}

	// Wait on the completed counter, not on the jobs: a job's Run returning does
	// not mean its duration has been recorded yet — that happens in the tracker's
	// defer, after Run returns.
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if findLane(t, p, LaneInteractive).Completed == 4 &&
			findLane(t, p, LaneInteractive).Running == 0 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	st = findLane(t, p, LaneInteractive)
	if !st.Durations.HasData {
		t.Fatal("HasData=false after 4 completed jobs")
	}
	if st.Durations.Samples != 4 {
		t.Errorf("samples=%d, want 4", st.Durations.Samples)
	}
	if st.Completed != 4 {
		t.Errorf("completed=%d, want 4", st.Completed)
	}
	if st.Durations.Median <= 0 || st.Durations.P99 <= 0 {
		t.Fatalf("median=%v p99=%v, want positive durations", st.Durations.Median, st.Durations.P99)
	}
	if st.Durations.Median > st.Durations.P99 {
		t.Errorf("median=%v exceeds p99=%v", st.Durations.Median, st.Durations.P99)
	}
	if st.Durations.P99 < 100*time.Millisecond {
		t.Errorf("p99=%v, want it to reflect the 120ms outlier", st.Durations.P99)
	}
	if st.Running != 0 {
		t.Errorf("running=%d after all jobs finished, want 0", st.Running)
	}
}

// Task 2.3: the duration ring is bounded, so a long-lived lane does not grow
// without limit.
func TestDurationRingIsBounded(t *testing.T) {
	st := &laneStat{}
	for i := 0; i < durationRingSize*3; i++ {
		st.observe(time.Duration(i)*time.Millisecond, nil)
	}
	if len(st.durations) != durationRingSize {
		t.Fatalf("ring holds %d durations, want %d", len(st.durations), durationRingSize)
	}
	if st.completed.Load() != int64(durationRingSize*3) {
		t.Errorf("completed=%d, want %d (counters are not bounded)",
			st.completed.Load(), durationRingSize*3)
	}
}

// Task 2.5: a job that ran and returned an error is failed; a job abandoned by
// cancellation is not.
func TestLaneClassifiesFailedAndAbandoned(t *testing.T) {
	p := New(Config{MaintenanceSize: 1, MaintenanceQueue: 4})
	defer p.Shutdown()

	boom := errors.New("upstream refused")
	fut, err := p.Enqueue(context.Background(), &Job{
		Lane:        LaneMaintenance,
		MaxAttempts: 1,
		Run:         func(ctx context.Context) error { return boom },
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := fut.Await(ctx); err == nil {
		t.Fatal("await returned nil for a failing job")
	}

	// Abandon this one: block the worker, queue a second, cancel the wait.
	release := make(chan struct{})
	var started sync.WaitGroup
	started.Add(1)
	if _, err := p.Enqueue(context.Background(), &Job{
		Lane: LaneMaintenance,
		Run: func(ctx context.Context) error {
			started.Done()
			<-release
			return nil
		},
	}); err != nil {
		t.Fatal(err)
	}
	started.Wait()

	abandonCtx, abandonCancel := context.WithCancel(context.Background())
	abandoned, err := p.Enqueue(context.Background(), &Job{
		Lane: LaneMaintenance,
		Run:  func(ctx context.Context) error { return errors.New("should not run") },
	})
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		_ = abandoned.Await(abandonCtx)
	}()
	abandonCancel()

	close(release)

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		st := findLane(t, p, LaneMaintenance)
		if st.Failed >= 1 && st.Abandoned >= 1 {
			if st.Failed != 1 {
				t.Errorf("failed=%d, want exactly 1", st.Failed)
			}
			if st.Abandoned != 1 {
				t.Errorf("abandoned=%d, want exactly 1", st.Abandoned)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	st := findLane(t, p, LaneMaintenance)
	t.Fatalf("failed=%d abandoned=%d, want both 1", st.Failed, st.Abandoned)
}

// Task 2.6: reading lane state while jobs run does not delay them.
func TestReadingLaneStateDoesNotDelayJobs(t *testing.T) {
	p := New(Config{InteractiveSize: 4, InteractiveQueue: 64})
	defer p.Shutdown()

	var done atomic.Bool
	var wg sync.WaitGroup
	wg.Add(1)
	if _, err := p.Enqueue(context.Background(), &Job{
		Lane: LaneInteractive,
		Run: func(ctx context.Context) error {
			defer wg.Done()
			// Poll state from another goroutine while this job is in flight.
			deadline := time.Now().Add(300 * time.Millisecond)
			for time.Now().Before(deadline) {
				_ = p.LaneStates()
				time.Sleep(time.Millisecond)
			}
			return nil
		},
	}); err != nil {
		t.Fatal(err)
	}

	finished := make(chan struct{})
	go func() { wg.Wait(); close(finished) }()
	select {
	case <-finished:
		done.Store(true)
	case <-time.After(5 * time.Second):
		t.Fatal("job did not finish while lane state was being polled")
	}
	if !done.Load() {
		t.Fatal("unreachable")
	}
}

// percentile interpolates and clamps at the ends.
func TestPercentileEdges(t *testing.T) {
	if got := percentile(nil, 0.5); got != 0 {
		t.Errorf("percentile(nil)=%v, want 0", got)
	}
	one := []time.Duration{5 * time.Second}
	if got := percentile(one, 0.99); got != 5*time.Second {
		t.Errorf("percentile(single)=%v, want 5s", got)
	}
	sorted := []time.Duration{time.Second, 2 * time.Second, 3 * time.Second, 4 * time.Second}
	if got := percentile(sorted, 0); got != time.Second {
		t.Errorf("percentile(p0)=%v, want 1s", got)
	}
	if got := percentile(sorted, 1); got != 4*time.Second {
		t.Errorf("percentile(p1)=%v, want 4s", got)
	}
}

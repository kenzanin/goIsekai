package workers

import (
	"context"
	"errors"
	"slices"
	"sync"
	"sync/atomic"
	"time"
)

// durationRingSize bounds how many recent durations a lane remembers. 128
// samples is enough for a stable p99 shape on a busy day and costs a few KB
// per lane; durations are not accumulated forever because the pool is meant to
// run for months.
const durationRingSize = 128

// laneStat is the observable state of one lane. Counters are atomic because
// workers write them concurrently with readers; the duration ring takes a
// mutex because it is a slice.
type laneStat struct {
	running   atomic.Int64
	completed atomic.Int64
	failed    atomic.Int64
	abandoned atomic.Int64

	mu        sync.Mutex
	durations []time.Duration // ring buffer, len capped at durationRingSize
	next      int
}

// observe records one finished job. A job that ended because its context was
// cancelled is abandoned, not failed: it never got to do the work, and counting
// it as a failure would make a lane that is quietly discarding cancelled work
// look like it is erroring.
func (s *laneStat) observe(d time.Duration, err error) {
	switch {
	case err == nil:
		s.completed.Add(1)
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		s.abandoned.Add(1)
	default:
		s.failed.Add(1)
	}

	s.mu.Lock()
	if s.durations == nil {
		s.durations = make([]time.Duration, 0, durationRingSize)
	}
	if len(s.durations) < durationRingSize {
		s.durations = append(s.durations, d)
	} else {
		s.durations[s.next] = d
		s.next = (s.next + 1) % durationRingSize
	}
	s.mu.Unlock()
}

// snapshot copies the ring out and summarises it. Reports HasData=false for an
// empty ring rather than a zero duration, which would read as instantaneous
// work.
func (s *laneStat) snapshot() DurationSummary {
	s.mu.Lock()
	out := make([]time.Duration, len(s.durations))
	copy(out, s.durations)
	s.mu.Unlock()

	if len(out) == 0 {
		return DurationSummary{}
	}
	slices.Sort(out)
	return DurationSummary{
		Samples: len(out),
		Median:  percentile(out, 0.50),
		P99:     percentile(out, 0.99),
		HasData: true,
	}
}

// percentile interpolates over a sorted slice. p is in [0,1].
func percentile(sorted []time.Duration, p float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	if len(sorted) == 1 {
		return sorted[0]
	}
	pos := p * float64(len(sorted)-1)
	lo := int(pos)
	hi := lo + 1
	if hi >= len(sorted) {
		return sorted[len(sorted)-1]
	}
	frac := pos - float64(lo)
	return sorted[lo] + time.Duration(frac*float64(sorted[hi]-sorted[lo]))
}

// DurationSummary reports how long work on a lane took. A lane that has not
// completed a job reports HasData=false and leaves the durations zero.
type DurationSummary struct {
	Samples int
	Median  time.Duration
	P99     time.Duration
	HasData bool
}

// LaneState is a point-in-time view of one lane. Waiting comes from the queue
// channel's length, so it cannot drift from what is actually queued.
type LaneState struct {
	Lane     Lane
	Waiting  int
	Running  int
	Capacity int

	Durations DurationSummary
	Completed int64
	Failed    int64
	Abandoned int64
}

// statFor returns the stat block for a lane. The image lane is two priority
// channels under one logical lane, so both feed the same block.
func (p *Pool) statFor(lane Lane) *laneStat {
	switch lane {
	case LaneInteractive:
		return &p.stats[LaneInteractive]
	case LaneFetch:
		return &p.stats[LaneFetch]
	case LaneImage:
		return &p.stats[LaneImage]
	case LaneMaintenance:
		return &p.stats[LaneMaintenance]
	}
	return &p.stats[LaneInteractive]
}

// LaneStates returns the observable state of all four lanes, in lane order.
// Every lane is always present, whether or not it has been used.
func (p *Pool) LaneStates() []LaneState {
	return []LaneState{
		p.laneState(LaneInteractive, len(p.interactive.queue), p.cfg.InteractiveSize, &p.stats[LaneInteractive]),
		p.laneState(LaneFetch, len(p.fetch.queue), p.cfg.FetchSize, &p.stats[LaneFetch]),
		p.laneState(LaneImage, len(p.imageHigh.queue)+len(p.imageLow.queue), p.cfg.ImageSize, &p.stats[LaneImage]),
		p.laneState(LaneMaintenance, len(p.maintenance.queue), p.cfg.MaintenanceSize, &p.stats[LaneMaintenance]),
	}
}

func (p *Pool) laneState(lane Lane, waiting, capacity int, st *laneStat) LaneState {
	return LaneState{
		Lane:      lane,
		Waiting:   waiting,
		Running:   int(st.running.Load()),
		Capacity:  capacity,
		Durations: st.snapshot(),
		Completed: st.completed.Load(),
		Failed:    st.failed.Load(),
		Abandoned: st.abandoned.Load(),
	}
}

// track wraps a job's execution so the lane's running count and duration ring
// stay accurate. Installed once per job in Enqueue, next to the retry wrapper.
//
// Classification uses the job's own error, not ctx.Err(): a job that ran and
// succeeded is completed even if its context is cancelled a moment later, and a
// job that ran and failed is failed even if nothing cancelled it.
func (p *Pool) track(h *jobHandle, next func(context.Context) error) func(context.Context) error {
	return func(ctx context.Context) (err error) {
		st := p.statFor(h.job.Lane)
		st.running.Add(1)
		start := time.Now()
		defer func() {
			st.running.Add(-1)
			st.observe(time.Since(start), err)
		}()
		return next(ctx)
	}
}

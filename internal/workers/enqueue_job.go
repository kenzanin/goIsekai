package workers

import (
	"context"
	"errors"
	"fmt"
	"time"

	"goisekai/internal/logger"
)

// Enqueue schedules a job. Backpressure (task 1.3):
//   - interactive: blocks until queued or ctx deadline → ErrEnqueueTimeout
//   - fetch/image/maintenance: blocks up to BackgroundEnqueueTimeout → ErrBusy
func (p *Pool) Enqueue(ctx context.Context, job *Job) (*Future, error) {
	if p.closed.Load() {
		return nil, ErrClosed
	}
	if job == nil || job.Run == nil {
		return nil, errors.New("workers: nil job")
	}
	if job.ID == "" {
		job.ID = fmt.Sprintf("%s-%d", job.Lane, p.seq.Add(1))
	}
	if job.MaxAttempts <= 0 {
		job.MaxAttempts = defaultMaxAttempts
	}

	h := &jobHandle{
		job:    job,
		fut:    &Future{pool: p},
		status: StatusQueued,
		run:    nil, // assigned below after wrap
	}
	h.fut.done = make(chan struct{})
	h.fut.info = h
	h.setPool(p) // link for status callbacks
	jctx, cancel := context.WithCancel(p.rootCtx)
	h.jctx = jctx
	h.cancel = cancel

	// Dedupe (design 3.2): while a job with this DedupeKey is queued or running,
	// a second enqueue returns that job's future instead of starting a duplicate.
	p.regMu.Lock()
	if job.DedupeKey != "" {
		if id, ok := p.dedupe[job.DedupeKey]; ok {
			if old, live := p.registry[id]; live && !old.fut.isDone() {
				p.regMu.Unlock()
				cancel()
				return old.fut, nil
			}
			delete(p.dedupe, job.DedupeKey) // stale: the job already finished
		}
		p.dedupe[job.DedupeKey] = job.ID
	}
	p.registry[job.ID] = h
	p.regMu.Unlock()
	logger.Debug("worker: enqueued", "jobID", job.ID, "lane", job.Lane.String(), "plugin", job.PluginKey)
	h.setStatus(StatusQueued) // triggers callback if set

	var lane *poolLane
	switch job.Lane {
	case LaneInteractive:
		lane = p.interactive
	case LaneFetch:
		lane = p.fetch
	case LaneImage:
		if job.Priority == PriorityHigh {
			lane = p.imageHigh
		} else {
			lane = p.imageLow
		}
	case LaneMaintenance:
		lane = p.maintenance
	case LaneCDP:
		lane = p.cdp
	default:
		cancel()
		return nil, fmt.Errorf("workers: unknown lane %d", int(job.Lane))
	}

	// Wrap Run with retry + registry bookkeeping once, here. Adds duration
	// tracking for worker job visibility.
	//
	// Every lifecycle record carries the lane as a structured attr
	// ("lane=fetch"), so the log viewer's lane filter has a single shape to
	// match. Only terminal states are loud: running/done are per-job chatter
	// at Debug, a retry is a Warn, and a job that exhausted its attempts is an
	// Error. Logging here is the sole output path — the old fmt.Println/stdout
	// copies wrote every job twice into the same file.
	h.run = p.track(h, func(ctx context.Context) error {
		start := time.Now()
		laneName := lane.Name()
		logger.Debug("worker: running", "jobID", job.ID, "lane", laneName, "plugin", job.PluginKey)
		h.setStatus(StatusRunning)
		var err error
		for attempt := 1; attempt <= job.MaxAttempts; attempt++ {
			h.setAttempt(attempt)
			if ctx.Err() != nil {
				h.setStatus(StatusDead)
				h.setErr(ctx.Err())
				return ctx.Err()
			}
			err = job.Run(ctx)
			if err == nil {
				logger.Debug("worker: done", "jobID", job.ID, "lane", laneName, "dur_ms", time.Since(start).Milliseconds())
				h.setStatus(StatusDone)
				h.setErr(nil)
				return nil
			}
			if _, ok := errors.AsType[*ErrPermanent](err); ok {
				break // permanent: no retry
			}
			if attempt < job.MaxAttempts {
				h.setStatus(StatusFailed)
				logger.Warn("worker: failed, retrying", "jobID", job.ID, "lane", laneName, "attempt", attempt)
				// Small linear backoff; transient-only.
				select {
				case <-time.After(time.Duration(attempt) * 50 * time.Millisecond):
				case <-ctx.Done():
					h.setStatus(StatusDead)
					h.setErr(ctx.Err())
					return ctx.Err()
				}
			}
		}
		logger.Error("worker: dead", "jobID", job.ID, "lane", laneName, "dur_ms", time.Since(start).Milliseconds(), "lastErr", err)
		h.setStatus(StatusDead)
		h.setErr(err)
		return err
	})

	if job.Lane == LaneInteractive {
		// Block until queued or ctx deadline (task 1.3).
		select {
		case lane.queue <- h:
			return h.fut, nil
		case <-ctx.Done():
			cancel()
			p.deregister(job.ID)
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				h.failQueued(ErrEnqueueTimeout)
				return nil, ErrEnqueueTimeout
			}
			h.failQueued(ctx.Err())
			return nil, ctx.Err()
		}
	}

	// Background lanes: short bounded block then ErrBusy.
	timer := time.NewTimer(p.cfg.BackgroundEnqueueTimeout)
	defer timer.Stop()
	select {
	case lane.queue <- h:
		return h.fut, nil
	case <-timer.C:
		cancel()
		p.deregister(job.ID)
		h.failQueued(ErrBusy)
		return nil, ErrBusy
	case <-ctx.Done():
		cancel()
		p.deregister(job.ID)
		h.failQueued(ctx.Err())
		return nil, ctx.Err()
	}
}

// failQueued completes a job's future after it was published to the registry and
// the dedupe map but never reached a lane queue.
//
// Enqueue publishes p.registry[job.ID] and p.dedupe[key] before pushing onto the
// lane, so a concurrent Enqueue with the same DedupeKey can be handed this
// future in that window. If the push then fails, deregister removes the entry and
// nothing will ever close the future's done channel - a follower would park in
// Await until its own context expired, or forever if it had no deadline.
// Completing the future turns that park into an error the caller can act on.
func (h *jobHandle) failQueued(err error) {
	h.setStatus(StatusDead)
	h.setErr(err)
	h.fut.once.Do(func() {
		h.fut.mu.Lock()
		h.fut.result = err
		h.fut.mu.Unlock()
		close(h.fut.done)
	})
}

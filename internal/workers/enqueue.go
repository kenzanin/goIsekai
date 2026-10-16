package workers

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"goisekai/internal/logger"
)

// New creates a Pool with the given sizing and starts all workers.
func New(cfg Config) *Pool {
	cfg = cfg.withDefaults()
	ctx, cancel := context.WithCancel(context.Background())
	p := &Pool{
		cfg:              cfg,
		interactive:      &poolLane{name: LaneInteractive, queue: make(chan *jobHandle, cfg.InteractiveQueue)},
		fetch:            &poolLane{name: LaneFetch, queue: make(chan *jobHandle, cfg.FetchQueue)},
		imageHigh:        &poolLane{name: LaneImage, queue: make(chan *jobHandle, cfg.ImageQueue)},
		imageLow:         &poolLane{name: LaneImage, queue: make(chan *jobHandle, cfg.ImageQueue)},
		maintenance:      &poolLane{name: LaneMaintenance, queue: make(chan *jobHandle, cfg.MaintenanceQueue)},
		cdp:              &poolLane{name: LaneCDP, queue: make(chan *jobHandle, cfg.CDPQueue)},
		fetchInUse:       map[string]bool{},
		interactiveInUse: map[string]bool{},
		registry:         map[string]*jobHandle{},
		detail:           map[string]string{},
		dedupe:           map[string]string{},
		rootCtx:          ctx,
		stop:             cancel,
	}
	for i := 0; i < cfg.InteractiveSize; i++ {
		p.spawnInteractive()
	}
	for i := 0; i < cfg.FetchSize; i++ {
		p.spawnFetch()
	}
	// Image workers drain high before low (design 1.1).
	for i := 0; i < cfg.ImageSize; i++ {
		p.spawnImage()
	}
	for i := 0; i < cfg.MaintenanceSize; i++ {
		p.spawnMaintenance()
	}
	for i := 0; i < cfg.CDPSize; i++ {
		p.spawnCDP()
	}
	return p
}

func (p *Pool) spawnInteractive() {
	p.nInteractive.Add(1)
	p.wg.Add(1)
	go p.worker(p.interactive, p.interactiveGate)
}

func (p *Pool) spawnFetch() {
	p.nFetch.Add(1)
	p.wg.Add(1)
	go p.worker(p.fetch, p.fetchGate)
}

func (p *Pool) spawnImage() {
	p.nImage.Add(1)
	p.wg.Add(1)
	go p.imageWorker()
}

func (p *Pool) spawnMaintenance() {
	p.nMaintenance.Add(1)
	p.wg.Add(1)
	go p.worker(p.maintenance, nil)
}

func (p *Pool) spawnCDP() {
	p.nCDP.Add(1)
	p.wg.Add(1)
	go p.worker(p.cdp, nil)
}

// ResizeLane grows one lane's worker count to n (no-op on shrink). Wired to
// the config reload path (task 1.4); grow-only by design — shrinking needs
// drain protocols nothing requires yet.
func (p *Pool) ResizeLane(lane Lane, n int) {
	if p.closed.Load() {
		return
	}
	for {
		var cur int64
		switch lane {
		case LaneInteractive:
			cur = p.nInteractive.Load()
		case LaneFetch:
			cur = p.nFetch.Load()
		case LaneImage:
			cur = p.nImage.Load()
		case LaneMaintenance:
			cur = p.nMaintenance.Load()
		case LaneCDP:
			cur = p.nCDP.Load()
		default:
			return
		}
		if cur >= int64(n) {
			return
		}
		switch lane {
		case LaneInteractive:
			if p.nInteractive.CompareAndSwap(cur, cur+1) {
				p.wg.Add(1)
				go p.worker(p.interactive, p.interactiveGate)
			}
		case LaneFetch:
			if p.nFetch.CompareAndSwap(cur, cur+1) {
				p.wg.Add(1)
				go p.worker(p.fetch, p.fetchGate)
			}
		case LaneImage:
			if p.nImage.CompareAndSwap(cur, cur+1) {
				p.wg.Add(1)
				go p.imageWorker()
			}
		case LaneMaintenance:
			if p.nMaintenance.CompareAndSwap(cur, cur+1) {
				p.wg.Add(1)
				go p.worker(p.maintenance, nil)
			}
		case LaneCDP:
			if p.nCDP.CompareAndSwap(cur, cur+1) {
				p.wg.Add(1)
				go p.worker(p.cdp, nil)
			}
		}
	}
}

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
	// All worker logs now carry [lane] prefix and include start, stop, dur_ms.
	h.run = p.track(h, wrappedJob(lane.Name(), job.ID, func(ctx context.Context) error {
		start := time.Now()
		fmt.Printf("%s [%s] jobID=%s running\n", start.Format("15:04:05"), lane.Name(), job.ID)
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
				dur := time.Since(start)
				fmt.Printf("%s [%s] jobID=%s done dur_ms=%d\n", time.Now().Format("15:04:05"), lane.Name(), job.ID, dur.Milliseconds())
				h.setStatus(StatusDone)
				h.setErr(nil)
				return nil
			}
			if _, ok := errors.AsType[*ErrPermanent](err); ok {
				break // permanent: no retry
			}
			if attempt < job.MaxAttempts {
				h.setStatus(StatusFailed)
				fmt.Printf("%s [%s] jobID=%s failed, retrying attempt=%d\n", time.Now().Format("15:04:05"), lane.Name(), job.ID, attempt)
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
		dur := time.Since(start)
		fmt.Printf("%s [%s] jobID=%s dead dur_ms=%d lastErr=%v\n", time.Now().Format("15:04:05"), lane.Name(), job.ID, dur.Milliseconds(), err)
		h.setStatus(StatusDead)
		h.setErr(err)
		return err
	}))

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

// takeKey implements per-plugin fairness for one lane: at most one in-flight
// job per PluginKey — the worker parks others at the queue tail. An empty key
// always passes.
func (p *Pool) takeKey(inUse map[string]bool, mu *sync.Mutex, key string) (take bool, release func()) {
	if key == "" {
		return true, func() {}
	}
	mu.Lock()
	if inUse[key] {
		mu.Unlock()
		return false, nil
	}
	inUse[key] = true
	mu.Unlock()
	return true, func() {
		mu.Lock()
		delete(inUse, key)
		mu.Unlock()
	}
}

// fetchGate enforces per-plugin fairness: returns false when another job for
// the same PluginKey is in flight — the worker then parks this job at the
// queue tail.
func (p *Pool) fetchGate(h *jobHandle) (take bool, release func()) {
	return p.takeKey(p.fetchInUse, &p.fetchMu, h.job.PluginKey)
}

// interactiveGate is fetchGate for the interactive lane (task 4.1).
func (p *Pool) interactiveGate(h *jobHandle) (take bool, release func()) {
	return p.takeKey(p.interactiveInUse, &p.interactiveMu, h.job.PluginKey)
}

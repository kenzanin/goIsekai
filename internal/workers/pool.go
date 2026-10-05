// Package workers provides bounded, typed worker lanes for the app's
// concurrent work: interactive UI, plugin fetch, image fetch, maintenance.
//
// Lanes are independent pools (design D1): a full image queue can never
// block the interactive lane. Jobs are typed (design D2) and carry a
// context-aware Future result; the context controls waiting AND cancellation
// of the job while it is still queued.
package workers

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// Lane identifies which pool a job runs on.
type Lane int

const (
	LaneInteractive Lane = iota
	LaneFetch
	LaneImage
	LaneMaintenance
)

func (l Lane) String() string {
	switch l {
	case LaneInteractive:
		return "interactive"
	case LaneFetch:
		return "fetch"
	case LaneImage:
		return "image"
	case LaneMaintenance:
		return "maintenance"
	}
	return fmt.Sprintf("lane(%d)", int(l))
}

// Priority orders the two image sub-queues (high drained before low).
type Priority int

const (
	PriorityLow Priority = iota
	PriorityHigh
)

// JobStatus is the observable lifecycle state (design D6).
type JobStatus int

const (
	StatusQueued JobStatus = iota
	StatusRunning
	StatusDone
	StatusFailed
	StatusDead
)

func (s JobStatus) String() string {
	switch s {
	case StatusQueued:
		return "queued"
	case StatusRunning:
		return "running"
	case StatusDone:
		return "done"
	case StatusFailed:
		return "failed"
	case StatusDead:
		return "dead"
	}
	return fmt.Sprintf("status(%d)", int(s))
}

var (
	// ErrBusy is returned when a background lane rejects a job within its
	// short enqueue timeout (callers map it to 503).
	ErrBusy = errors.New("workers: queue full")
	// ErrClosed is returned after Shutdown.
	ErrClosed = errors.New("workers: pool closed")
	// ErrEnqueueTimeout is returned by the interactive lane when its ctx
	// deadline hits before the queue frees up (callers map it to 503).
	ErrEnqueueTimeout = errors.New("workers: enqueue timed out")
)

// ErrPermanent marks errors that must not be retried (rate limit exhausted,
// permanent upstream 404...). Wrap with Permanent().
type ErrPermanent struct{ Err error }

func (e *ErrPermanent) Error() string { return e.Err.Error() }
func (e *ErrPermanent) Unwrap() error { return e.Err }

// Permanent wraps err so the retry logic stops retrying it.
func Permanent(err error) error {
	if err == nil {
		return nil
	}
	return &ErrPermanent{Err: err}
}

// Job is one unit of work on a lane. Run may be retried on transient errors
// (anything not wrapped in Permanent) up to MaxAttempts total tries.
type Job struct {
	ID          string
	Lane        Lane
	Priority    Priority // image lane only
	PluginKey   string   // fetch lane: fairness key, one in-flight per key
	DedupeKey   string   // when set, a second enqueue returns the in-flight job
	Run         func(ctx context.Context) error
	MaxAttempts int // <=0 means default (3)
}

// JobInfo is the registry view of a job (design D6).
type JobInfo struct {
	ID        string
	Lane      Lane
	Status    JobStatus
	Attempts  int
	LastError error
}

// Future is a context-aware result placeholder (design D2). Await honors
// ctx: when ctx is cancelled the caller stops waiting and the job is
// cancelled if still queued — no result is delivered.
type Future struct {
	done   chan struct{}
	once   sync.Once
	mu     sync.Mutex
	result error
	info   *jobHandle
	pool   *Pool // back-reference for status callbacks
}

// jobHandle couples a Future with its cancellation + registry entry.
type jobHandle struct {
	job      *Job
	fut      *Future
	cancel   context.CancelFunc
	jctx     context.Context
	run      func(ctx context.Context) error
	mu       sync.Mutex
	status   JobStatus
	attempts int
	lastErr  error
}

// GetID returns the job ID. Safe to call before and after completion.
func (f *Future) GetID() string {
	if f == nil || f.info == nil || f.info.job == nil {
		return ""
	}
	return f.info.job.ID
}

// Await blocks until the job finishes or ctx is cancelled. Returns the job
// error (nil on success), ctx.Err() if waiting was abandoned (the queued job
// is cancelled), or ErrClosed after Shutdown.
// isDone reports whether the job has already delivered its result.
func (f *Future) isDone() bool {
	select {
	case <-f.done:
		return true
	default:
		return false
	}
}

func (f *Future) Await(ctx context.Context) error {
	select {
	case <-f.done:
		f.mu.Lock()
		defer f.mu.Unlock()
		return f.result
	case <-ctx.Done():
		// Abandon: cancel the queued job so it does not run later and
		// never deliver a result to this waiter.
		if f.info != nil {
			f.info.cancel()
		}
		return ctx.Err()
	}
}

// Config sizes the lanes. Zero values mean "use the code default".
type Config struct {
	InteractiveSize  int // worker count, default 4
	InteractiveQueue int
	FetchSize        int // default 2
	FetchQueue       int // default 8
	ImageSize        int // default 8
	ImageQueue       int // default 64 (shared by both priorities)
	MaintenanceSize  int // default 1 (serial)
	MaintenanceQueue int
	// BackgroundEnqueueTimeout bounds how long a background lane enqueue
	// blocks before ErrBusy. Default 500ms.
	BackgroundEnqueueTimeout time.Duration
}

const (
	defaultInteractiveSize  = 4
	defaultInteractiveQueue = 32
	defaultFetchSize        = 2
	defaultFetchQueue       = 8
	defaultImageSize        = 8
	defaultImageQueue       = 64
	defaultMaintenanceSize  = 1
	defaultMaintenanceQueue = 8
	defaultBGEnqueueTimeout = 500 * time.Millisecond
	defaultMaxAttempts      = 3
)

func (c Config) withDefaults() Config {
	if c.InteractiveSize <= 0 {
		c.InteractiveSize = defaultInteractiveSize
	}
	if c.InteractiveQueue <= 0 {
		c.InteractiveQueue = defaultInteractiveQueue
	}
	if c.FetchSize <= 0 {
		c.FetchSize = defaultFetchSize
	}
	if c.FetchQueue <= 0 {
		c.FetchQueue = defaultFetchQueue
	}
	if c.ImageSize <= 0 {
		c.ImageSize = defaultImageSize
	}
	if c.ImageQueue <= 0 {
		c.ImageQueue = defaultImageQueue
	}
	if c.MaintenanceSize <= 0 {
		c.MaintenanceSize = defaultMaintenanceSize
	}
	if c.MaintenanceQueue <= 0 {
		c.MaintenanceQueue = defaultMaintenanceQueue
	}
	if c.BackgroundEnqueueTimeout <= 0 {
		c.BackgroundEnqueueTimeout = defaultBGEnqueueTimeout
	}
	return c
}

// poolLane is one bounded queue + its workers. The queue is typed Job
// pointers; typing happens at the enqueue API (callers pass a Job with a
// typed Run closure).
type poolLane struct {
	name  Lane
	queue chan *jobHandle
}

// Pool owns the four lanes and the job registry.
type Pool struct {
	cfg Config

	interactive *poolLane
	fetch       *poolLane
	imageHigh   *poolLane
	imageLow    *poolLane
	maintenance *poolLane

	// fetchInUse enforces per-plugin fairness: at most one running job per
	// PluginKey (design: FetchWorker).
	fetchInUse map[string]bool
	fetchMu    sync.Mutex
	// interactiveInUse applies the same per-plugin fairness to the interactive
	// lane (task 4.1): a stalled plugin invoke holds one worker plus its VM
	// mutex instead of parking sibling invokes on extra workers.
	interactiveInUse map[string]bool
	interactiveMu    sync.Mutex

	registry map[string]*jobHandle
	detail   map[string]string // job ID -> result detail (e.g. export path)
	dedupe   map[string]string // DedupeKey -> in-flight job ID (design 3.2)
	regMu    sync.Mutex

	// stats is one block per Lane, indexed by the Lane constant. See stats.go.
	stats [4]laneStat

	nInteractive atomic.Int64
	nFetch       atomic.Int64
	nImage       atomic.Int64
	nMaintenance atomic.Int64

	seq     atomic.Uint64
	closed  atomic.Bool
	wg      sync.WaitGroup
	rootCtx context.Context
	stop    context.CancelFunc

	// StatusChanged is called when a job's status changes (queued/running/done/failed).
	// ID, status and any result detail are passed; the callback runs synchronously
	// during the state update.
	StatusChanged func(id string, status JobStatus, detail string)
}

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
	default:
		cancel()
		return nil, fmt.Errorf("workers: unknown lane %d", int(job.Lane))
	}

	// Wrap Run with retry + registry bookkeeping once, here.
	wrapped := func(ctx context.Context) error {
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
				h.setStatus(StatusDone)
				h.setErr(nil)
				return nil
			}
			if _, ok := errors.AsType[*ErrPermanent](err); ok {
				break // permanent: no retry
			}
			if attempt < job.MaxAttempts {
				h.setStatus(StatusFailed)
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
		h.setStatus(StatusDead)
		h.setErr(err)
		return err
	}

	// Store wrapped on the handle so the worker calls it. track() records the
	// lane's running count and duration; it sits outside the retry wrapper so
	// the duration measured is wall-clock including retries.
	h.run = p.track(h, wrapped)

	if job.Lane == LaneInteractive {
		// Block until queued or ctx deadline (task 1.3).
		select {
		case lane.queue <- h:
			return h.fut, nil
		case <-ctx.Done():
			cancel()
			p.deregister(job.ID)
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return nil, ErrEnqueueTimeout
			}
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
		return nil, ErrBusy
	case <-ctx.Done():
		cancel()
		p.deregister(job.ID)
		return nil, ctx.Err()
	}
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

// worker runs jobs from one lane sequentially-per-slot. gate, when non-nil,
// can veto a job (fetch fairness); a vetoed job goes back to the queue tail.
func (p *Pool) worker(lane *poolLane, gate func(*jobHandle) (bool, func())) {
	defer p.wg.Done()
	for {
		select {
		case <-p.rootCtx.Done():
			return
		case h := <-lane.queue:
			release := func() {}
			if gate != nil {
				requeued := false
				for {
					take, rel := gate(h)
					if take {
						release = rel
						break
					}
					// Same PluginKey in flight: park at the queue tail for
					// fairness. If the tail is full of same-key siblings the
					// put fails — sleep-retry instead of spinning or
					// dropping the job.
					select {
					case lane.queue <- h:
						requeued = true // back to the tail; work the next handle
					case <-p.rootCtx.Done():
						return
					case <-time.After(time.Millisecond):
						// retry the gate
					}
					if requeued {
						break
					}
				}
				if requeued {
					continue // outer job loop: dequeue the next handle
				}
			}
			// Error rides the Future (lastErrSnapshot below); callers Await it.
			_ = h.run(h.jctx)
			release()
			// Deliver result exactly once.
			h.fut.once.Do(func() {
				h.fut.mu.Lock()
				h.fut.result = h.lastErrSnapshot()
				h.fut.mu.Unlock()
				close(h.fut.done)
			})
			// ponytail: keep registry entries ~5min then drop — Jobs are
			// short-lived; add a janitor when status pages need history.
			p.deregister(h.job.ID)
		}
	}
}

// imageWorker drains high-priority before low-priority jobs.
func (p *Pool) imageWorker() {
	defer p.wg.Done()
	for {
		// Prefer high, non-blocking.
		select {
		case <-p.rootCtx.Done():
			return
		case h := <-p.imageHigh.queue:
			p.runHandle(h)
			continue
		default:
		}
		select {
		case <-p.rootCtx.Done():
			return
		case h := <-p.imageHigh.queue:
			p.runHandle(h)
		case h := <-p.imageLow.queue:
			p.runHandle(h)
		}
	}
}

func (p *Pool) runHandle(h *jobHandle) {
	// Error rides the Future (lastErrSnapshot below); callers Await it.
	_ = h.run(h.jctx)
	h.fut.once.Do(func() {
		h.fut.mu.Lock()
		h.fut.result = h.lastErrSnapshot()
		h.fut.mu.Unlock()
		close(h.fut.done)
	})
	p.deregister(h.job.ID)
}

// Status returns registry info for a job id.
func (p *Pool) Status(id string) (JobInfo, bool) {
	p.regMu.Lock()
	defer p.regMu.Unlock()
	h, ok := p.registry[id]
	if !ok {
		return JobInfo{}, false
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	return JobInfo{ID: id, Lane: h.job.Lane, Status: h.status, Attempts: h.attempts, LastError: h.lastErr}, true
}

// Shutdown stops accepting jobs and cancels queued work. Running jobs are
// cancelled via ctx; workers exit. Safe to call twice.
func (p *Pool) Shutdown() {
	if p.closed.Swap(true) {
		return
	}
	p.stop()
	// Unblock workers sitting on full/empty queues.
	p.wg.Wait()
}

// --- jobHandle internals ---

func (h *jobHandle) setStatus(s JobStatus) {
	h.mu.Lock()
	h.status = s
	h.mu.Unlock()
	// Notify status change callback if set (design D6).
	if h.job != nil && h.job.ID != "" && h.fut != nil && h.fut.info == h {
		if cb := h.getPool().StatusChanged; cb != nil {
			cb(h.job.ID, s, h.getPool().detailOf(h.job.ID))
		}
	}
}

// getPool returns the Pool owning this job handle via the Future.
func (h *jobHandle) getPool() *Pool { return h.fut.getPool() }

func (f *Future) getPool() *Pool {
	if f == nil {
		return nil
	}
	return f.pool
}

// setPool links the Future to its Pool for callbacks.
func (h *jobHandle) setPool(p *Pool) {
	if h.fut != nil {
		h.fut.pool = p
	}
}

func (h *jobHandle) setAttempt(n int) {
	h.mu.Lock()
	h.attempts = n
	h.mu.Unlock()
}

func (h *jobHandle) setErr(err error) {
	h.mu.Lock()
	h.lastErr = err
	h.mu.Unlock()
}

func (h *jobHandle) lastErrSnapshot() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.lastErr
}

func (p *Pool) deregister(id string) {
	p.regMu.Lock()
	if h, ok := p.registry[id]; ok && h.job.DedupeKey != "" && p.dedupe[h.job.DedupeKey] == id {
		delete(p.dedupe, h.job.DedupeKey)
	}
	delete(p.registry, id)
	delete(p.detail, id)
	p.regMu.Unlock()
}

// SetDetail records a result detail for a job (e.g. the finished CBZ path) so
// status callbacks can announce it. Safe to call from inside a job's Run.
func (p *Pool) SetDetail(id, detail string) {
	p.regMu.Lock()
	p.detail[id] = detail
	p.regMu.Unlock()
}

func (p *Pool) detailOf(id string) string {
	p.regMu.Lock()
	defer p.regMu.Unlock()
	return p.detail[id]
}

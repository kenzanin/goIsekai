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

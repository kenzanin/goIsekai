package workers

import (
	"context"
)

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

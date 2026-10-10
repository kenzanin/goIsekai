package workers

import (
	"time"
)

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

// poolDrainTimeout bounds how long Shutdown waits for running jobs to exit
// after cancellation. ponytail: jobs that honor their ctx (image/fetch lanes)
// drain in milliseconds; this ceiling exists for jobs that don't — the
// challenge-solve engine ladder runs its per-engine timeouts under a
// Background context, which used to block shutdown for minutes (one stuck
// onisaga solve froze SIGTERM indefinitely). Raise/remove it once hostnet
// propagates the job ctx into the solver. A var so tests can shrink it.
var poolDrainTimeout = 5 * time.Second

// Shutdown stops accepting jobs and cancels queued work. Running jobs are
// cancelled via ctx; workers exit. Safe to call twice.
func (p *Pool) Shutdown() {
	if p.closed.Swap(true) {
		return
	}
	p.stop()
	// Unblock workers sitting on full/empty queues.
	drained := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(drained)
	}()
	select {
	case <-drained:
	case <-time.After(poolDrainTimeout):
		// Workers still busy with a non-cooperative job: proceed with the
		// process shutdown anyway — exiting reaps them.
	}
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

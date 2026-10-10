package workers

import (
	"time"
)

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

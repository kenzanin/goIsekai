package workers

import (
	"sync"
)

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

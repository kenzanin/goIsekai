package hostnet

import "time"

func engineFailed(engine string) {
	engineHealth.Lock()
	defer engineHealth.Unlock()
	engineHealth.lastFail[engine] = time.Now()
}

func engineSucceeded(engine string) {
	engineHealth.Lock()
	defer engineHealth.Unlock()
	delete(engineHealth.lastFail, engine)
}

// engineTripped reports whether engine failed within the last engineCoolDown.
func engineTripped(engine string) bool {
	engineHealth.Lock()
	defer engineHealth.Unlock()
	t, ok := engineHealth.lastFail[engine]
	return ok && time.Since(t) < engineCoolDown
}

// fallbackChain returns the ordered engine names to attempt for a solve: the
// configured engine first, then the "obscura" and "lightpanda" daemons. An
// engine is only included when it has an endpoint of the right shape — a
// daemon without a ws:// URL could only fail, and including it would consume
// an attempt that a working engine needs. Duplicates are removed and disabled
// engines ("", "off") are dropped.
func (c CDPConfig) fallbackChain() []string {
	candidates := []string{c.Engine, "obscura", "lightpanda"}
	chain := make([]string, 0, len(candidates))
	seen := make(map[string]bool, len(candidates))
	for _, e := range candidates {
		if e == "" || e == "off" || seen[e] || !c.usable(e) {
			continue
		}
		seen[e] = true
		chain = append(chain, e)
	}
	return chain
}

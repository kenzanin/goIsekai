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

// cdpFallbackChain returns the ordered engine names to attempt for a solve:
// the configured engine first, then the "obscura" and "lightpanda" daemon
// engines as fallbacks. Duplicates are removed (the configured engine is not
// repeated when it is already one of the fallbacks) and disabled engines
// ("", "off") are dropped.
func cdpFallbackChain(configured string) []string {
	candidates := []string{configured, "obscura", "lightpanda"}
	chain := make([]string, 0, len(candidates))
	seen := make(map[string]bool, len(candidates))
	for _, e := range candidates {
		if e == "" || e == "off" || seen[e] {
			continue
		}
		seen[e] = true
		chain = append(chain, e)
	}
	return chain
}

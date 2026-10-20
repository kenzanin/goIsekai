package hostnet

import (
	"context"
	"reflect"
	"testing"
	"time"

	fhttp "github.com/bogdanfinn/fhttp"
)

// TestFallbackChainSkipsEnginesWithoutEndpoint locks in the fix for the
// fallback chain handing a chrome binary path to a daemon engine.
//
// The regression: cdpFallbackChain("chrome") returned [chrome obscura
// lightpanda] and solveChallenge swapped only cfg.Engine, leaving Path as the
// chrome binary. obscura then failed instantly with `cdp_path must be a ws://
// URL`, which consumed the only fallback attempt (the `tried >= 2` budget) and
// meant lightpanda was never reached at all.
func TestFallbackChainSkipsEnginesWithoutEndpoint(t *testing.T) {
	chromeOnly := CDPConfig{Engine: "chrome", Path: "/usr/bin/google-chrome-stable"}
	if got, want := chromeOnly.fallbackChain(), []string{"chrome"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("chrome-only chain = %v, want %v (daemons have no endpoint)", got, want)
	}

	// A daemon with its own endpoint joins the chain, and it is the only
	// fallback worth an attempt.
	withObscura := CDPConfig{
		Engine: "chrome",
		Path:   "/usr/bin/google-chrome-stable",
		Paths:  map[string]string{"obscura": "ws://127.0.0.1:9222"},
	}
	if got, want := withObscura.fallbackChain(), []string{"chrome", "obscura"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("chain = %v, want %v", got, want)
	}

	// A daemon endpoint with the wrong shape is not usable either.
	wrongShape := CDPConfig{
		Engine: "chrome",
		Path:   "/usr/bin/google-chrome-stable",
		Paths:  map[string]string{"lightpanda": "/usr/bin/lightpanda"},
	}
	if got, want := wrongShape.fallbackChain(), []string{"chrome"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("chain = %v, want %v (daemon path is not ws://)", got, want)
	}

	// Disabled stays empty.
	if got := (CDPConfig{Engine: "off"}).fallbackChain(); len(got) != 0 {
		t.Fatalf("off chain = %v, want empty", got)
	}
}

// TestSolveChallengeUsesPerEngineEndpoint records the endpoint actually handed
// to each engine, so a future refactor cannot silently pass chrome's binary to
// a daemon again.
func TestSolveChallengeUsesPerEngineEndpoint(t *testing.T) {
	cfg := CDPConfig{
		Engine: "chrome",
		Path:   "/usr/bin/google-chrome-stable",
		Paths:  map[string]string{"obscura": "ws://127.0.0.1:9222"},
	}
	if got := cfg.pathFor("obscura"); got != "ws://127.0.0.1:9222" {
		t.Fatalf("obscura endpoint = %q, want its own ws:// URL", got)
	}
	if got := cfg.pathFor("chrome"); got != "/usr/bin/google-chrome-stable" {
		t.Fatalf("chrome endpoint = %q, want the binary path", got)
	}
	if got := cfg.pathFor("lightpanda"); got != "" {
		t.Fatalf("unconfigured lightpanda endpoint = %q, want empty", got)
	}
}

// TestEngineCoolDownClears keeps the circuit breaker honest: a tripped engine
// is skipped, and a success clears the entry.
func TestEngineCoolDownClears(t *testing.T) {
	const engine = "obscura-test-cooldown"
	engineSucceeded(engine)
	if engineTripped(engine) {
		t.Fatal("engine should not be tripped after a success")
	}
	engineFailed(engine)
	if !engineTripped(engine) {
		t.Fatal("engine should be tripped right after a failure")
	}
	engineSucceeded(engine)
	if engineTripped(engine) {
		t.Fatal("engineSucceeded must clear the failure entry")
	}
	// Avoid leaking a fail entry into other tests.
	engineHealth.Lock()
	delete(engineHealth.lastFail, engine)
	engineHealth.Unlock()
}

// captureTimeoutSolver records the CDPConfig handed to the solver so a test
// can assert the timeout actually used.
type captureTimeoutSolver struct {
	cfg   CDPConfig
	calls int
}

func (c *captureTimeoutSolver) solve(cfg CDPConfig, _ string) ([]*fhttp.Cookie, string, error) {
	c.calls++
	c.cfg = cfg
	return []*fhttp.Cookie{{Name: "cf_clearance", Value: "x"}}, "ua", nil
}

// TestSolveAndSeedClampsTimeoutToContext locks in the fix for CDP solves
// outliving their caller. cdp_solve_timeout is often 120s while a plugin
// invoke is capped at 15-30s, so an unclamped solve burned the full engine
// timeout (twice: primary + one fallback) for a caller that had already given
// up, surfacing only "context deadline exceeded".
func TestSolveAndSeedClampsTimeoutToContext(t *testing.T) {
	p := NewProxy()
	p.ConfigureCDP(CDPConfig{Engine: "chrome", Path: "/usr/bin/chrome", Timeout: 120 * time.Second})
	solver := &captureTimeoutSolver{}
	p.solveChallenge = solver.solve

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := p.solveAndSeed(ctx, "plugin-1", "https://example.com/page"); err != nil {
		t.Fatalf("solveAndSeed: %v", err)
	}
	if solver.calls != 1 {
		t.Fatalf("solver calls = %d, want 1", solver.calls)
	}
	if solver.cfg.Timeout > 2*time.Second {
		t.Fatalf("solver timeout = %s, want it clamped to the 2s ctx budget", solver.cfg.Timeout)
	}
	if solver.cfg.Timeout <= 0 {
		t.Fatalf("solver timeout = %s, want a positive budget", solver.cfg.Timeout)
	}

	// Without a deadline the configured timeout is left alone.
	p2 := NewProxy()
	p2.ConfigureCDP(CDPConfig{Engine: "chrome", Path: "/usr/bin/chrome", Timeout: 120 * time.Second})
	solver2 := &captureTimeoutSolver{}
	p2.solveChallenge = solver2.solve
	if err := p2.solveAndSeed(context.Background(), "plugin-1", "https://example.com/page"); err != nil {
		t.Fatalf("solveAndSeed (no deadline): %v", err)
	}
	if solver2.cfg.Timeout != 120*time.Second {
		t.Fatalf("solver timeout = %s, want the configured 120s when ctx has no deadline", solver2.cfg.Timeout)
	}
}

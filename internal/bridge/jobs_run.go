package bridge

import (
	"context"
	"errors"

	"goisekai/internal/hostnet"
	"goisekai/internal/workers"
)

// permanentIfFinal marks an error that retrying cannot fix, so the lane's retry
// loop stops instead of re-running the job to MaxAttempts. Only the anti-bot
// challenge qualifies today: it needs a human to open VerifyURL, so re-invoking
// the plugin reproduces it exactly and just hammers the site. Everything else
// stays retryable, which is why this is the one producer of workers.Permanent.
func permanentIfFinal(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := errors.AsType[*hostnet.ChallengeError](err); ok {
		return workers.Permanent(err)
	}
	return err
}

// runOnFetch routes a request-shaped fetch unit through the fetch lane so
// per-plugin fairness applies (one in-flight job per PluginKey), then waits
// for its result. Library sync stays fire-and-forget at the enqueue seam; the
// paths that need a value back (task 3.1: migration candidate search,
// enrichment fetch, cover refetch) join the lane instead of bypassing it.
func (s *AppService) runOnFetch(ctx context.Context, pluginKey string, fn func(context.Context) error) error {
	fut, err := s.pool.Enqueue(ctx, &workers.Job{
		Lane:      workers.LaneFetch,
		PluginKey: pluginKey,
		Run: func(ctx context.Context) error {
			return permanentIfFinal(fn(ctx))
		},
	})
	if err != nil {
		return err
	}
	return fut.Await(ctx)
}

// runOnInteractive routes a UI-facing plugin invoke through the interactive
// lane (task 4.1) with the same per-plugin fairness as fetch, then waits for
// its result. Enqueue failures surface workers.ErrEnqueueTimeout/ErrBusy
// (task 4.2 → 503); plugin failures flow through as ordinary errors so the
// callers' cache fallbacks keep working.
//
// The fn receives an exec-bounded context so a hung plugin call cannot
// hog an interactive worker forever. If the call exceeds
// interactiveExecTimeout it returns context.DeadlineExceeded, which the
// caller treats as a transient failure (cache fallback).
func (s *AppService) runOnInteractive(ctx context.Context, pluginKey string, fn func(context.Context) error) error {
	enqueueCtx, enqueueCancel := context.WithTimeout(ctx, interactiveEnqueueTimeout)
	defer enqueueCancel()
	fut, err := s.pool.Enqueue(enqueueCtx, &workers.Job{
		Lane:        workers.LaneInteractive,
		PluginKey:   pluginKey,
		MaxAttempts: 1, // request-shaped: fail fast, no lane-level retry
		Run: func(ctx context.Context) error {
			// Bounded execution context: does not inherit the HTTP request
			// context's lifetime — only the exec timeout. The request context
			// still gates Enqueue (above) so a cancelled request can't start work.
			//
			// Most plugin bodies ignore the ctx they receive, so a plain
			// WithTimeout cannot free the lane on its own: race the call
			// against the deadline and give up waiting when it expires. The
			// abandoned goroutine finishes on its own once the underlying
			// invoke returns; the lane slot and fairness key are released now.
			execCtx, execCancel := context.WithTimeout(context.Background(), interactiveExecTimeout)
			defer execCancel()
			done := make(chan error, 1)
			go func() { done <- fn(execCtx) }()
			select {
			case err := <-done:
				return err
			case <-execCtx.Done():
				return execCtx.Err()
			}
		},
	})
	if err != nil {
		return err
	}
	// Bound the wait: a job parked behind a hung same-plugin sibling must
	// not translate into an unbounded request hang. Expiry cancels the
	// queued job (Await semantics) and surfaces ctx.DeadlineExceeded,
	// which callers map to their cache fallback.
	awaitCtx, awaitCancel := context.WithTimeout(ctx, interactiveAwaitTimeout)
	defer awaitCancel()
	return fut.Await(awaitCtx)
}

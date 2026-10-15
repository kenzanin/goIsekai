package bridge

import (
	"context"
	"errors"
	"time"

	"goisekai/internal/workers"
	"goisekai/pkg/types"
)

// interactiveEnqueueTimeout bounds how long a UI invoke waits for a queue
// slot before the handler surfaces 503 (design D6) — the request never
// blocks indefinitely on a full lane. 5s: shorter than a reader's patience, longer than a queue blip.
const interactiveEnqueueTimeout = 5 * time.Second

// interactiveExecTimeout bounds how long the plugin call itself may run inside
// the worker. This is independent of enqueue timeout: a job may wait in queue
// < enqueue timeout, but once running it must not hog the lane. 30s covers a
// worst-case CDP challenge solve; anything longer is treated as hung.
const interactiveExecTimeout = 30 * time.Second

// interactiveAwaitTimeout bounds how long a caller blocks on the job's future:
// enqueue (5s) + exec (30s) + 5s queue slack. Without it a request whose job is
// parked behind a hung same-plugin sibling waits forever — that is exactly how
// one stuck plugin invoke used to hang the whole UI. Expiry surfaces as
// context.DeadlineExceeded, which callers map to their cache fallback.
const interactiveAwaitTimeout = interactiveEnqueueTimeout + interactiveExecTimeout + 5*time.Second

// EnqueueExportCBZ queues a CBZ export on the image lane. Returns a job ID for
// tracking; the finished path is announced via the job status callback so the
// WS/toast surface can point the user at the file.
func (s *AppService) EnqueueExportCBZ(ctx context.Context, pluginID, mangaID, chapterID, title string) (string, error) {
	job := &workers.Job{
		Lane:      workers.LaneImage,
		DedupeKey: "export:" + pluginID + ":" + mangaID + ":" + chapterID,
	}
	job.Run = func(ctx context.Context) error {
		path, exportErr := s.ExportCBZ(pluginID, mangaID, chapterID, title)
		if exportErr == nil {
			s.pool.SetDetail(job.ID, path)
		}
		return exportErr
	}
	fut, err := s.pool.Enqueue(ctx, job)
	if err != nil {
		return "", err
	}
	return fut.GetID(), nil
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
		Run:       fn,
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

// IsQueueFull reports lane-level backpressure (interactive enqueue timeout or
// background busy) — handlers map it to 503. Plugin-level errors must not
// classify as this so callers' cache fallbacks stay intact.
func IsQueueFull(err error) bool {
	return errors.Is(err, workers.ErrEnqueueTimeout) || errors.Is(err, workers.ErrBusy)
}

// EnqueueSearchManga queues a SearchManga call on the fetch lane with callback.
// Returns a job ID for tracking; the callback runs after job completion with
// the results. This is for fan-out patterns where multiple sources are searched
// in parallel (e.g. migration candidate picker).
func (s *AppService) EnqueueSearchManga(ctx context.Context, pluginID string, filter types.SearchFilter, cb func(context.Context, []types.Manga, error)) (*workers.Future, error) {
	job := &workers.Job{
		Lane:      workers.LaneFetch,
		PluginKey: pluginID,
	}
	job.Run = func(ctx context.Context) error {
		results, err := s.SearchManga(pluginID, filter)
		cb(ctx, results, err)
		return nil
	}
	return s.pool.Enqueue(ctx, job)
}

// EnqueueGetChapterList queues a GetChapterList call on the fetch lane with callback.
// Returns a job ID for tracking; the callback runs after job completion with
// the results. This is for fan-out patterns where multiple chapter counts are
// fetched in parallel (e.g. migration candidate picker).
func (s *AppService) EnqueueGetChapterList(ctx context.Context, pluginID, mangaID string, cb func(context.Context, []types.Chapter, error)) (*workers.Future, error) {
	job := &workers.Job{
		Lane:      workers.LaneFetch,
		PluginKey: pluginID,
	}
	job.Run = func(ctx context.Context) error {
		chapters, err := s.GetChapterList(pluginID, mangaID)
		cb(ctx, chapters, err)
		return nil
	}
	return s.pool.Enqueue(ctx, job)
}

// EnqueueMarkChapterComplete queues a MarkChapterComplete call on the interactive lane.
// Returns a job ID for tracking. This is for progress completion marking without
// blocking the UI thread.
func (s *AppService) EnqueueMarkChapterComplete(ctx context.Context, pluginID, mangaID, chapterID string) (string, error) {
	job := &workers.Job{
		Lane:        workers.LaneInteractive,
		PluginKey:   pluginID,
		MaxAttempts: 1, // no retry for completion marking
	}
	job.Run = func(ctx context.Context) error {
		return s.MarkChapterComplete(pluginID, mangaID, chapterID)
	}
	fut, err := s.pool.Enqueue(ctx, job)
	if err != nil {
		return "", err
	}
	return fut.GetID(), nil
}

// EnqueueSetChapterProgress queues a chapter progress update on the interactive lane.
// If the chapter is complete (lastPage >= total), it also enqueues MarkChapterComplete.
// Returns a job ID for tracking. This is for progress updates without blocking the UI thread.
func (s *AppService) EnqueueSetChapterProgress(ctx context.Context, pluginID, mangaID, chapterID string, lastPage int) (string, error) {
	job := &workers.Job{
		Lane:        workers.LaneInteractive,
		PluginKey:   pluginID,
		MaxAttempts: 1, // no retry for progress update
	}
	job.Run = func(ctx context.Context) error {
		return s.SetChapterProgress(pluginID, mangaID, chapterID, lastPage)
	}
	fut, err := s.pool.Enqueue(ctx, job)
	if err != nil {
		return "", err
	}
	return fut.GetID(), nil
}

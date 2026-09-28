package bridge

import (
	"context"
	"errors"
	"time"

	"goisekai/internal/workers"
)

// interactiveEnqueueTimeout bounds how long a UI invoke waits for a queue
// slot before the handler surfaces 503 (design D6) — the request never
// blocks indefinitely on a full lane. Job execution itself is not cancelled
// by it. 5s: shorter than a reader's patience, longer than a queue blip.
const interactiveEnqueueTimeout = 5 * time.Second

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
func (s *AppService) runOnInteractive(ctx context.Context, pluginKey string, fn func(context.Context) error) error {
	enqueueCtx, cancel := context.WithTimeout(ctx, interactiveEnqueueTimeout)
	defer cancel()
	fut, err := s.pool.Enqueue(enqueueCtx, &workers.Job{
		Lane:        workers.LaneInteractive,
		PluginKey:   pluginKey,
		MaxAttempts: 1, // request-shaped: fail fast, no lane-level retry
		Run:         fn,
	})
	if err != nil {
		return err
	}
	return fut.Await(ctx)
}

// IsQueueFull reports lane-level backpressure (interactive enqueue timeout or
// background busy) — handlers map it to 503. Plugin-level errors must not
// classify as this so callers' cache fallbacks stay intact.
func IsQueueFull(err error) bool {
	return errors.Is(err, workers.ErrEnqueueTimeout) || errors.Is(err, workers.ErrBusy)
}

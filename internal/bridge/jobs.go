package bridge

import (
	"context"
	"errors"
	"time"

	"goisekai/internal/workers"
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
		path, exportErr := s.ExportCBZ(pluginID, mangaID, chapterID, title, func(pct int) {
			s.pool.SetProgress(job.ID, pct)
		})
		if exportErr == nil {
			s.pool.SetDetail(job.ID, path)
		}
		return permanentIfFinal(exportErr)
	}
	fut, err := s.pool.Enqueue(ctx, job)
	if err != nil {
		return "", err
	}
	return fut.GetID(), nil
}

// IsQueueFull reports lane-level backpressure (interactive enqueue timeout or
// background busy) — handlers map it to 503. Plugin-level errors must not
// classify as this so callers' cache fallbacks stay intact.
func IsQueueFull(err error) bool {
	return errors.Is(err, workers.ErrEnqueueTimeout) || errors.Is(err, workers.ErrBusy)
}

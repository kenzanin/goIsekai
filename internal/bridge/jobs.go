package bridge

import (
	"context"
	"goisekai/internal/workers"
)

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

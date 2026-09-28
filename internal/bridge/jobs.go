package bridge

import (
	"context"
	"goisekai/internal/workers"
)

// EnqueueExportCBZ queues a CBZ export on the image lane. Returns a job ID for
// tracking; the finished path is announced via the job status callback so the
// WS/toast surface can point the user at the file.
func (s *AppService) EnqueueExportCBZ(ctx context.Context, pluginID, mangaID, chapterID, title string) (string, error) {
	job := &workers.Job{Lane: workers.LaneImage}
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


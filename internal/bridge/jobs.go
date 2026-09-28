package bridge

import (
	"context"
	"goisekai/internal/workers"
)

// JobStatusResult is the JSON response for a job status query.
type JobStatusResult struct {
	ID        string          `json:"id"`
	Lane      string          `json:"lane"`
	Status    string          `json:"status"`
	Attempts  int             `json:"attempts"`
	LastError string          `json:"lastError,omitempty"`
}

// GetJobStatus returns the status of a job by ID.
func (s *AppService) GetJobStatus(id string) (JobStatusResult, bool) {
	info, ok := s.pool.Status(id)
	if !ok {
		return JobStatusResult{}, false
	}
	lastErr := ""
	if info.LastError != nil {
		lastErr = info.LastError.Error()
	}
	return JobStatusResult{
		ID:        info.ID,
		Lane:      info.Lane.String(),
		Status:    info.Status.String(),
		Attempts:  info.Attempts,
		LastError: lastErr,
	}, true
}

// EnqueueExportCBZ queues a CBZ export on the image lane.
// Returns a job ID for tracking.
func (s *AppService) EnqueueExportCBZ(ctx context.Context, pluginID, mangaID, chapterID, title string) (string, error) {
	fut, err := s.pool.Enqueue(ctx, &workers.Job{
		Lane: workers.LaneImage,
		Run: func(ctx context.Context) error {
			_, exportErr := s.ExportCBZ(pluginID, mangaID, chapterID, title)
			return exportErr
		},
	})
	if err != nil {
		return "", err
	}
	return fut.GetID(), nil
}


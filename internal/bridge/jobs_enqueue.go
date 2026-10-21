package bridge

import (
	"context"

	"goisekai/internal/workers"
	"goisekai/pkg/types"
)

// enqueueFanout is the shared plumbing behind EnqueueSearchManga and
// EnqueueGetChapterList: fetch lane, plugin key, and the plugin error handed to
// the callback instead of the job. The job reports nil regardless, so the retry
// loop has nothing to retry — the fan-out callers (migration candidate picker)
// collect failures through cb themselves.
func enqueueFanout[T any](s *AppService, ctx context.Context, pluginID string, call func() (T, error), cb func(context.Context, T, error)) (*workers.Future, error) {
	job := &workers.Job{
		Lane:      workers.LaneFetch,
		PluginKey: pluginID,
	}
	job.Run = func(ctx context.Context) error {
		result, err := call()
		cb(ctx, result, err)
		return nil
	}
	return s.pool.Enqueue(ctx, job)
}

// EnqueueSearchManga queues a SearchManga call on the fetch lane with callback.
// Returns a job ID for tracking; the callback runs after job completion with
// the results. This is for fan-out patterns where multiple sources are searched
// in parallel (e.g. migration candidate picker).
func (s *AppService) EnqueueSearchManga(ctx context.Context, pluginID string, filter types.SearchFilter, cb func(context.Context, []types.Manga, error)) (*workers.Future, error) {
	return enqueueFanout(s, ctx, pluginID, func() ([]types.Manga, error) {
		return s.SearchManga(pluginID, filter)
	}, cb)
}

// EnqueueGetChapterList queues a GetChapterList call on the fetch lane with callback.
// Returns a job ID for tracking; the callback runs after job completion with
// the results. This is for fan-out patterns where multiple chapter counts are
// fetched in parallel (e.g. migration candidate picker).
func (s *AppService) EnqueueGetChapterList(ctx context.Context, pluginID, mangaID string, cb func(context.Context, []types.Chapter, error)) (*workers.Future, error) {
	return enqueueFanout(s, ctx, pluginID, func() ([]types.Chapter, error) {
		return s.GetChapterList(pluginID, mangaID)
	}, cb)
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

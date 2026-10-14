package bridge

import (
	"context"
	"time"

	"goisekai/internal/logger"
	"goisekai/internal/workers"
)

// StartLibraryScheduler re-syncs stale library manga once an hour until the
// returned stop channel is closed. A manga is stale when its updated_at is
// older than update_stale_days (minimum 1) — the threshold is re-read from
// goisekai.ini every tick, so edits take effect without a restart.
func StartLibraryScheduler(s *AppService, stop <-chan struct{}, pool *workers.Pool) {
	go func() {
		ctx := context.Background()
		t := time.NewTicker(time.Hour)
		defer t.Stop()
		for {
			select {
			case <-stop:
				return
			case <-t.C:
				days := s.updateStaleDays()
				job := &workers.Job{
					Lane: workers.LaneMaintenance,
					Run: func(ctx context.Context) error {
						return s.SyncStaleLibrary(time.Now().AddDate(0, 0, -days))
					},
				}
				if _, err := pool.Enqueue(ctx, job); err != nil {
					logger.Error("scheduled library sync enqueue", "error", err)
				}
			}
		}
	}()
}

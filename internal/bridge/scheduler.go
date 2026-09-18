package bridge

import (
	"time"

	"goisekai/internal/logger"
)

// StartLibraryScheduler re-syncs stale library manga once an hour until the
// returned stop channel is closed. A manga is stale when its updated_at is
// older than update_stale_days (minimum 1) — the threshold is re-read from
// goisekai.ini every tick, so edits take effect without a restart.
func StartLibraryScheduler(s *AppService, stop <-chan struct{}) {
	go func() {
		t := time.NewTicker(time.Hour)
		defer t.Stop()
		for {
			select {
			case <-stop:
				return
			case <-t.C:
				days := s.updateStaleDays()
				if err := s.SyncStaleLibrary(time.Now().AddDate(0, 0, -days)); err != nil {
					logger.Error("scheduled library sync", "error", err)
				}
			}
		}
	}()
}

package bridge

import (
	"fmt"
	"time"

	"goisekai/internal/database"
	"goisekai/internal/logger"
)

// SyncStaleLibrary re-syncs only the in-library manga whose updated_at is
// older than cutoff — the hourly scheduler's pass.
func (s *AppService) SyncStaleLibrary(cutoff time.Time) error {
	stale, err := s.db.ListLibraryStale(cutoff)
	if err != nil {
		return fmt.Errorf("bridge: sync stale library: %w", err)
	}
	if len(stale) == 0 {
		return nil
	}
	logger.Debug("scheduled library refresh", "stale", len(stale))
	return s.syncMangas(stale)
}

// syncMangas refreshes each manga in the batch, skipping plugins that are
// switched off and continuing past individual failures so one dead source
// cannot abort the whole pass.
func (s *AppService) syncMangas(library []database.Manga) error {
	for _, manga := range library {
		if !s.db.IsPluginActive(manga.PluginID) {
			continue
		}
		m, detailErr := s.mgr.GetMangaDetail(manga.PluginID, manga.SourceMangaID)
		if detailErr != nil {
			logger.Error("sync detail failed", "id", manga.ID, "plugin", manga.PluginID, "error", detailErr)
			continue
		}
		chapters, chapErr := s.mgr.GetChapterList(manga.PluginID, manga.SourceMangaID)
		if chapErr != nil {
			logger.Error("sync chapters failed", "id", manga.ID, "error", chapErr)
			continue
		}
		prevCount, _ := s.db.CountChaptersForManga(manga.ID)
		if persistErr := s.persistMangaDetails(manga.PluginID, m, chapters); persistErr != nil {
			logger.Error("sync persist failed", "id", manga.ID, "error", persistErr)
			continue
		}
		// Stamp the [New] badge when this sync discovered chapters we didn't have.
		if n, cerr := s.db.CountChaptersForManga(manga.ID); cerr == nil && n > prevCount {
			if nerr := s.db.MarkMangaNew(manga.ID); nerr != nil {
				logger.Warn("mark manga new", "id", manga.ID, "error", nerr)
			}
		}
	}
	return nil
}

package bridge

import (
	"fmt"
	"time"

	"goisekai/internal/database"
	"goisekai/internal/logger"
	"goisekai/pkg/types"
)

// SyncLibrary re-fetches chapter lists from source plugins for every manga in the library.
func (s *AppService) SyncLibrary() error {
	library, err := s.db.ListLibrary()
	if err != nil {
		return fmt.Errorf("bridge: sync library: %w", err)
	}
	return s.syncMangas(library)
}

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
	logger.Info("scheduled library refresh", "stale", len(stale))
	return s.syncMangas(stale)
}

func (s *AppService) syncMangas(library []database.Manga) error {
	for _, manga := range library {
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

// persistMangaDetails mirrors a fetched manga and its chapters into SQLite. The
// manga row id is an auto-increment integer, so the plugin and source ids live in
// their own columns and chapters reference the row id the upsert returns.
// Chapters are kept only for library manga: a non-library row is detail-view
// cache whose chapter list is re-fetched from the plugin on open.
func (s *AppService) persistMangaDetails(pluginID string, m types.Manga, chapters []types.Chapter) error {
	mangaIntID, err := s.db.UpsertManga(database.Manga{
		PluginID:      pluginID,
		SourceMangaID: m.ID,
		Title:         m.Title,
		CoverURL:      m.CoverURL,
		Description:   m.Description,
		Status:        m.Status,
	})
	if err != nil {
		return err
	}

	inLibrary, err := s.db.IsInLibrary(mangaIntID)
	if err != nil {
		return err
	}
	if !inLibrary {
		return nil
	}

	for _, c := range chapters {
		if _, err := s.db.UpsertChapter(database.Chapter{
			MangaID:         mangaIntID,
			SourceChapterID: c.ID,
			Title:           c.Title,
			ChapterNum:      c.ChapterNum,
			VolumeNum:       c.VolumeNum,
			FetchedAt:       c.ReleasedAt,
		}); err != nil {
			return err
		}
	}
	return nil
}

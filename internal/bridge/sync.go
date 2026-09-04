package bridge

import (
	"fmt"

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

// persistMangaDetails mirrors a fetched manga and its chapters into SQLite.
//
// database.Manga.ID is set to a stable, globally-unique key derived from the
// plugin id and source manga id. UpsertManga takes the caller-provided ID as
// the row id (it returns no generated id), and the row id is a plain TEXT
// primary key unique across every plugin — qualifying it with the plugin id
// keeps distinct sources from colliding while still letting the upsert's
// UNIQUE(plugin_id, source_manga_id) conflict clause do its job. The same key
// is reused as each chapter's manga_id, so the row id is known without a
// read-back query (ListLibrary can't be used for that: it filters in_library=1,
// but a freshly upserted manga is in_library=0).
func (s *AppService) persistMangaDetails(pluginID string, m types.Manga, chapters []types.Chapter) error {
	rowID := mangaRowID(pluginID, m.ID)
	if err := s.db.UpsertManga(database.Manga{
		ID:            rowID,
		PluginID:      pluginID,
		SourceMangaID: m.ID,
		Title:         m.Title,
		CoverURL:      m.CoverURL,
		Description:   m.Description,
		Status:        m.Status,
	}); err != nil {
		return err
	}
	for _, c := range chapters {
		if err := s.db.UpsertChapter(database.Chapter{
			ID:              mangaRowID(rowID, c.ID),
			MangaID:         rowID,
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

// mangaRowID builds the stable database primary key for a source row:
// "<pluginID>|<sourceID>". The pipe separator cannot appear in a plugin id
// (it is a trimmed base filename) and is extremely unlikely in a source id,
// keeping the hierarchy unambiguous. Upgrade to a delimiter-free scheme only if
// a source id is ever observed containing '|'.
func mangaRowID(pluginID, sourceID string) string {
	return pluginID + "|" + sourceID
}

// chapterRowID builds the database primary key for a chapter row:
// "<pluginID>|<sourceMangaID>|<sourceChapterID>".
func chapterRowID(pluginID, mangaID, chapterID string) string {
	return mangaRowID(pluginID, mangaID) + "|" + chapterID
}

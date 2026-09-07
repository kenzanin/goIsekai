package bridge

import (
	"encoding/json"
	"fmt"

	"goisekai/internal/database"
	"goisekai/internal/logger"
	"goisekai/pkg/types"
)

// SearchManga delegates to the plugin's Search function.
func (s *AppService) SearchManga(pluginID string, filter types.SearchFilter) ([]types.Manga, error) {
	result, err := s.mgr.Search(pluginID, filter)
	if err != nil {
		return nil, fmt.Errorf("bridge: search manga: %w", err)
	}
	return result, nil
}

// IsInLibrary reports whether a manga (by source ids) is in the library.
func (s *AppService) IsInLibrary(pluginID, mangaID string) bool {
	ok, err := s.db.IsInLibrary(mangaRowID(pluginID, mangaID))
	if err != nil {
		return false
	}
	return ok
}

// ClearMangaNew resets the library card's [New] badge once the manga is opened.
func (s *AppService) ClearMangaNew(pluginID, mangaID string) error {
	return s.db.ClearMangaNew(pluginID, mangaID)
}

// GetMangaDetails fetches a manga and its chapter list from a plugin, persists
// both to the database as a side effect (so progress can be tracked later),
// and returns the original plugin types unchanged.
func (s *AppService) GetMangaDetails(pluginID, mangaID string) (types.Manga, []types.Chapter, error) {
	manga, err := s.mgr.GetMangaDetail(pluginID, mangaID)
	if err != nil {
		return types.Manga{}, nil, fmt.Errorf("bridge: get manga detail: %w", err)
	}
	chapters, err := s.mgr.GetChapterList(pluginID, mangaID)
	if err != nil {
		return types.Manga{}, nil, fmt.Errorf("bridge: get chapter list: %w", err)
	}
	if err := s.persistMangaDetails(pluginID, manga, chapters); err != nil {
		return types.Manga{}, nil, fmt.Errorf("bridge: persist manga details: %w", err)
	}
	// A user-set main title wins over the plugin-sourced one.
	if dbTitle, custom, err := s.db.MangaTitleIfCustom(pluginID, mangaID); err == nil && custom {
		manga.Title = dbTitle
	}
	return manga, chapters, nil
}

// resolveChapterRowID maps a source chapter ID to the database row ID used
// by the chapters table ("pluginID|mangaID|sourceChapterID"). On failure it
// returns "" and the caller skips caching — best-effort only.
func (s *AppService) resolveChapterRowID(pluginID, chapterID string) string {
	rowID, err := s.db.ResolveChapterID(pluginID, chapterID)
	if err != nil {
		return ""
	}
	return rowID
}

// GetPageList delegates to the plugin's GetPageList function.
// On success the result is persisted to chapter_pages so a later
// GetPageListCached call can serve it when the plugin is unreachable.
func (s *AppService) GetPageList(pluginID, chapterID string) ([]types.Page, error) {
	result, err := s.mgr.GetPageList(pluginID, chapterID)
	if err != nil {
		return nil, fmt.Errorf("bridge: get page list: %w", err)
	}
	// Best-effort cache persist — log-and-ignore on failure.
	if raw, merr := json.Marshal(result); merr == nil {
		if rowID := s.resolveChapterRowID(pluginID, chapterID); rowID != "" {
			if perr := s.db.SaveChapterPages(rowID, raw); perr != nil {
				logger.Warn("cache chapter pages", "chapter", chapterID, "error", perr)
			}
		}
	}
	return result, nil
}

// GetPageListCached is like GetPageList but falls back to the local
// chapter_pages cache when the plugin call fails. A cache hit logs the
// original plugin error and returns the cached pages; a cache miss
// returns the original error unchanged.
func (s *AppService) GetPageListCached(pluginID, chapterID string) ([]types.Page, error) {
	result, err := s.mgr.GetPageList(pluginID, chapterID)
	if err == nil {
		// Online path succeeded — persist for future offline use.
		if raw, merr := json.Marshal(result); merr == nil {
			if rowID := s.resolveChapterRowID(pluginID, chapterID); rowID != "" {
				if perr := s.db.SaveChapterPages(rowID, raw); perr != nil {
					logger.Warn("cache chapter pages", "chapter", chapterID, "error", perr)
				}
			}
		}
		return result, nil
	}
	// Plugin failed — try the local cache.
	rowID := s.resolveChapterRowID(pluginID, chapterID)
	if rowID == "" {
		return nil, fmt.Errorf("bridge: get page list: %w", err)
	}
	cached, cerr := s.db.GetChapterPages(rowID)
	if cerr != nil {
		logger.Warn("read chapter pages cache", "chapter", chapterID, "error", cerr)
		return nil, fmt.Errorf("bridge: get page list: %w", err)
	}
	if cached != nil {
		logger.Warn("serving cached page list (plugin unreachable)", "chapter", chapterID, "plugin_err", err)
		var pages []types.Page
		if uerr := json.Unmarshal(cached, &pages); uerr != nil {
			return nil, fmt.Errorf("bridge: unmarshal cached pages: %w", uerr)
		}
		return pages, nil
	}
	return nil, fmt.Errorf("bridge: get page list: %w", err)
}

// ToggleLibraryItem flips the in-library flag for a manga, addressed by its
// source identifiers (pluginID + source manga id) rather than the internal
// database row id. The bridge reconstructs the row id internally so the
// frontend never needs to know the storage key scheme.
func (s *AppService) ToggleLibraryItem(pluginID, mangaID string) error {
	if err := s.db.ToggleLibrary(mangaRowID(pluginID, mangaID)); err != nil {
		return fmt.Errorf("bridge: toggle library item: %w", err)
	}
	return nil
}

// ListLibrary returns the user's in-library manga, most recently updated first.
func (s *AppService) ListLibrary() ([]database.Manga, error) {
	list, err := s.db.ListLibrary()
	if err != nil {
		return nil, fmt.Errorf("bridge: list library: %w", err)
	}
	return list, nil
}

package bridge

import (
	"fmt"
	"github.com/goccy/go-json"
	"slices"

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
// and returns the original plugin types unchanged. When the plugin is
// unreachable (e.g. network offline, site down), falls back to cached DB
// data so the detail page still renders.
func (s *AppService) GetMangaDetails(pluginID, mangaID string) (types.Manga, []types.Chapter, error) {
	rowID := mangaRowID(pluginID, mangaID)

	// When the plugin manager is nil (e.g. test environment), fall back to cache.
	if s.mgr == nil {
		return s.cachedMangaFallback(pluginID, mangaID, rowID)
	}

	// Try live fetch first.
	manga, err := s.mgr.GetMangaDetail(pluginID, mangaID)
	if err == nil {
		// Preserve raw plugin genres before any override.
		manga.RawGenres = make([]string, len(manga.Genres))
		copy(manga.RawGenres, manga.Genres)
		chapters, chapErr := s.mgr.GetChapterList(pluginID, mangaID)
		if chapErr == nil {
			if persistErr := s.persistMangaDetails(pluginID, manga, chapters); persistErr != nil {
				logger.Warn("persist manga details", "error", persistErr)
			}
			// A user-set main title wins over the plugin-sourced one.
			if dbTitle, custom, err := s.db.MangaTitleIfCustom(pluginID, mangaID); err == nil && custom {
				manga.Title = dbTitle
			}
			// A user-set main description (via alt-summary swap) wins over the plugin-sourced one.
			if dbDesc, custom, err := s.db.MangaDescriptionIfCustom(pluginID, mangaID); err == nil && custom {
				manga.Description = dbDesc
			}
			// A user-set genre override wins over the plugin-sourced list.
			if genres, has, err := s.GetMangaGenres(pluginID, mangaID); err == nil && has && len(genres) > 0 {
				manga.Genres = genres
			}
			// A user-set cover dim override wins over the default.
			if dim, has, _ := s.db.GetMangaCoverDim(rowID); has && dim == 1 {
				manga.CoverDim = dim
			}
			return manga, chapters, nil
		}
		// Chapter list failed — still persist manga alone.
		if persistErr := s.persistMangaDetails(pluginID, manga, nil); persistErr != nil {
			logger.Warn("persist manga details", "error", persistErr)
		}
		if dbTitle, custom, err := s.db.MangaTitleIfCustom(pluginID, mangaID); err == nil && custom {
			manga.Title = dbTitle
		}
		if dbDesc, custom, err := s.db.MangaDescriptionIfCustom(pluginID, mangaID); err == nil && custom {
			manga.Description = dbDesc
		}
		// A user-set genre override wins over the plugin-sourced list.
		if genres, has, err := s.GetMangaGenres(pluginID, mangaID); err == nil && has && len(genres) > 0 {
			manga.Genres = genres
		}
		// A user-set cover dim override wins over the default.
		if dim, has, _ := s.db.GetMangaCoverDim(rowID); has && dim == 1 {
			manga.CoverDim = dim
		}
		// Return live manga but fall back chapters.
		mangaChapters := s.liveChaptersFallback(rowID, chapters)
		return manga, mangaChapters, nil
	}

	// Plugin unreachable — fall back to cache.
	logger.Warn("plugin unreachable, using cached data", "plugin", pluginID, "manga", mangaID, "error", err)
	return s.cachedMangaFallback(pluginID, mangaID, rowID)
}

// liveChaptersFallback merges live chapters with DB progress when chapter
// fetch fails. Returns DB-backed chapters with live chapter IDs for read tracking.
func (s *AppService) liveChaptersFallback(rowID string, liveChapters []types.Chapter) []types.Chapter {
	if len(liveChapters) > 0 {
		return liveChapters
	}
	// No live chapters — fall back to DB cache.
	dbChapters, err := s.db.ListChaptersCached(rowID)
	if err != nil || len(dbChapters) == 0 {
		return nil
	}
	// Map DB chapters to types.Chapter.
	out := make([]types.Chapter, len(dbChapters))
	for i, c := range dbChapters {
		out[i] = types.Chapter{
			ID:         c.SourceChapterID,
			MangaID:    mangaIDFromRow(c.MangaID),
			Title:      c.Title,
			ChapterNum: c.ChapterNum,
			VolumeNum:  c.VolumeNum,
			ReleasedAt: c.FetchedAt,
		}
	}
	return out
}

// cachedMangaFallback returns cached manga + chapters from the database.
func (s *AppService) cachedMangaFallback(pluginID, mangaID, rowID string) (types.Manga, []types.Chapter, error) {
	// Fetch cached manga.
	cachedManga, err := s.db.GetMangaCached(pluginID, mangaID)
	if err != nil {
		return types.Manga{}, nil, fmt.Errorf("bridge: get manga detail: %w", err)
	}
	if cachedManga.ID == "" {
		return types.Manga{}, nil, fmt.Errorf(
			"bridge: plugin %s is unreachable and manga %s has no cached copy", pluginID, mangaID)
	}

	// Convert DB manga to types.Manga.
	manga := types.Manga{
		ID:          cachedManga.SourceMangaID,
		Title:       cachedManga.Title,
		CoverURL:    cachedManga.CoverURL,
		Description: cachedManga.Description,
		Status:      cachedManga.Status,
	}

	// Apply user overrides.
	if dbTitle, custom, err := s.db.MangaTitleIfCustom(pluginID, mangaID); err == nil && custom {
		manga.Title = dbTitle
	}
	if dbDesc, custom, err := s.db.MangaDescriptionIfCustom(pluginID, mangaID); err == nil && custom {
		manga.Description = dbDesc
	}
	// Apply user genre override.
	if genres, has, err := s.GetMangaGenres(pluginID, mangaID); err == nil && has && len(genres) > 0 {
		manga.Genres = genres
	}
	// Apply user cover dim override.
	if dim, has, _ := s.db.GetMangaCoverDim(rowID); has && dim == 1 {
		manga.CoverDim = dim
	}

	// Fetch cached chapters.
	dbChapters, err := s.db.ListChaptersCached(rowID)
	if err != nil || len(dbChapters) == 0 {
		return manga, nil, nil
	}

	// Map DB chapters to types.Chapter.
	chapters := make([]types.Chapter, len(dbChapters))
	for i, c := range dbChapters {
		chapters[i] = types.Chapter{
			ID:         c.SourceChapterID,
			MangaID:    mangaID,
			Title:      c.Title,
			ChapterNum: c.ChapterNum,
			VolumeNum:  c.VolumeNum,
			ReleasedAt: c.FetchedAt,
		}
	}

	return manga, chapters, nil
}

// mangaIDFromRow extracts the source mangaID from a DB rowID.
func mangaIDFromRow(rowID string) string {
	for i := len(rowID) - 1; i >= 0; i-- {
		if rowID[i] == '|' {
			return rowID[i+1:]
		}
	}
	return rowID
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
		logger.Debug("page list cache: miss (online success)", "chapter", chapterID, "pages", len(result))
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
	logger.Debug("page list cache: plugin failed, trying local cache", "chapter", chapterID, "plugin_err", err)
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
		logger.Info("page list cache: hit (serving cached)", "chapter", chapterID)
		var pages []types.Page
		if uerr := json.Unmarshal(cached, &pages); uerr != nil {
			return nil, fmt.Errorf("bridge: unmarshal cached pages: %w", uerr)
		}
		return pages, nil
	}
	logger.Debug("page list cache: miss (no cached data)", "chapter", chapterID)
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

// SetMangaGenres stores a user-defined genre override for a manga.
// Pass nil to clear the override (return to plugin-supplied genres).
func (s *AppService) SetMangaGenres(pluginID, mangaID string, genres []string) error {
	rowID := mangaRowID(pluginID, mangaID)
	if err := s.db.SetMangaGenres(rowID, genres); err != nil {
		return fmt.Errorf("bridge: set manga genres: %w", err)
	}
	return nil
}

// GetMangaGenres returns the stored genre override for a manga.
// Returns the genres and true when an override exists; (nil, false) otherwise.
func (s *AppService) GetMangaGenres(pluginID, mangaID string) ([]string, bool, error) {
	rowID := mangaRowID(pluginID, mangaID)
	genres, has, err := s.db.GetMangaGenres(rowID)
	if err != nil {
		return nil, false, fmt.Errorf("bridge: get manga genres: %w", err)
	}
	return genres, has, nil
}

// AddGenre appends a genre to a manga's user-defined override.
// Creates the override from the current plugin-supplied genres if none exists.
func (s *AppService) AddGenre(pluginID, mangaID, genre string) error {
	rowID := mangaRowID(pluginID, mangaID)
	genres, has, err := s.db.GetMangaGenres(rowID)
	if err != nil {
		return fmt.Errorf("bridge: add genre: %w", err)
	}
	if !has {
		genres = nil
	}
	if slices.Contains(genres, genre) {
		return nil
	}
	genres = append(genres, genre)
	if err := s.db.SetMangaGenres(rowID, genres); err != nil {
		return fmt.Errorf("bridge: add genre: %w", err)
	}
	return nil
}

// ToggleGenre adds a genre to the override if not present, or removes it if present.
// Acts as a toggle: click adds → click removes.
func (s *AppService) ToggleGenre(pluginID, mangaID, genre string) error {
	rowID := mangaRowID(pluginID, mangaID)
	genres, has, err := s.db.GetMangaGenres(rowID)
	if err != nil {
		return fmt.Errorf("bridge: toggle genre: %w", err)
	}
	if !has {
		genres = nil
	}
	// Check if genre is already in the override
	for i, g := range genres {
		if g == genre {
			// Remove it
			genres = append(genres[:i], genres[i+1:]...)
			if len(genres) == 0 {
				genres = nil
			}
			if err := s.db.SetMangaGenres(rowID, genres); err != nil {
				return fmt.Errorf("bridge: toggle genre: %w", err)
			}
			return nil
		}
	}
	// Not in override — add it
	genres = append(genres, genre)
	if err := s.db.SetMangaGenres(rowID, genres); err != nil {
		return fmt.Errorf("bridge: toggle genre: %w", err)
	}
	return nil
}

// RemoveGenre removes one genre from a manga's user-defined override.
// If no override exists, it's a no-op (the X button won't appear).
func (s *AppService) RemoveGenre(pluginID, mangaID, genre string) error {
	rowID := mangaRowID(pluginID, mangaID)
	genres, has, err := s.db.GetMangaGenres(rowID)
	if err != nil {
		return fmt.Errorf("bridge: remove genre: %w", err)
	}
	if !has || genres == nil {
		return nil
	}
	filtered := make([]string, 0, len(genres))
	for _, g := range genres {
		if g != genre {
			filtered = append(filtered, g)
		}
	}
	if err := s.db.SetMangaGenres(rowID, filtered); err != nil {
		return fmt.Errorf("bridge: remove genre: %w", err)
	}
	return nil
}

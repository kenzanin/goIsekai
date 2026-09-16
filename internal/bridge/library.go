package bridge

import (
	"fmt"

	"goisekai/internal/database"
	"goisekai/internal/logger"
	"goisekai/pkg/types"

	"github.com/goccy/go-json"
	"slices"
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
	mangaIntID, _ := s.db.ResolveMangaIntID(pluginID, mangaID)
	ok, err := s.db.IsInLibrary(mangaIntID)
	if err != nil {
		return false
	}
	return ok
}

// ClearMangaNew resets the library card's [New] badge once the manga is opened.
func (s *AppService) ClearMangaNew(pluginID, mangaID string) error {
	return s.db.ClearMangaNewString(pluginID, mangaID)
}

// GetMangaDetails fetches a manga and its chapter list from a plugin.
func (s *AppService) GetMangaDetails(pluginID, mangaID string) (types.Manga, []types.Chapter, error) {
	mangaIntID, _ := s.db.ResolveMangaIntID(pluginID, mangaID)

	if s.mgr == nil {
		return s.cachedMangaFallback(pluginID, mangaID, mangaIntID)
	}

	manga, err := s.mgr.GetMangaDetail(pluginID, mangaID)
	if err == nil {
		manga.Genres = s.genres.normalize(manga.Genres)
		manga.RawGenres = make([]string, len(manga.Genres))
		copy(manga.RawGenres, manga.Genres)
		chapters, chapErr := s.mgr.GetChapterList(pluginID, mangaID)
		if chapErr == nil {
			if persistErr := s.persistMangaDetails(pluginID, manga, chapters); persistErr != nil {
				logger.Warn("persist manga details", "error", persistErr)
			}
			if dbTitle, custom, err := s.db.MangaTitleIfCustom(pluginID, mangaID); err == nil && custom {
				manga.Title = dbTitle
			}
			if dbDesc, custom, err := s.db.MangaDescriptionIfCustom(pluginID, mangaID); err == nil && custom {
				manga.Description = dbDesc
			}
			if genres, has, err := s.GetMangaGenres(pluginID, mangaID); err == nil && has && len(genres) > 0 {
				manga.Genres = genres
			}
			if dim, has, _ := s.db.GetMangaCoverDim(mangaIntID); has && dim == 1 {
				manga.CoverDim = dim
			}
			return manga, chapters, nil
		}
		if persistErr := s.persistMangaDetails(pluginID, manga, nil); persistErr != nil {
			logger.Warn("persist manga details", "error", persistErr)
		}
		if dbTitle, custom, err := s.db.MangaTitleIfCustom(pluginID, mangaID); err == nil && custom {
			manga.Title = dbTitle
		}
		if dbDesc, custom, err := s.db.MangaDescriptionIfCustom(pluginID, mangaID); err == nil && custom {
			manga.Description = dbDesc
		}
		if genres, has, err := s.GetMangaGenres(pluginID, mangaID); err == nil && has && len(genres) > 0 {
			manga.Genres = genres
		}
		if dim, has, _ := s.db.GetMangaCoverDim(mangaIntID); has && dim == 1 {
			manga.CoverDim = dim
		}
		return manga, s.liveChaptersFallback(mangaIntID, chapters), nil
	}

	logger.Warn("plugin unreachable, using cached data", "plugin", pluginID, "manga", mangaID)
	return s.cachedMangaFallback(pluginID, mangaID, mangaIntID)
}

// liveChaptersFallback returns cached chapters when live fetch fails.
func (s *AppService) liveChaptersFallback(mangaIntID int64, liveChapters []types.Chapter) []types.Chapter {
	if len(liveChapters) > 0 {
		return liveChapters
	}
	dbChapters, err := s.db.ListChaptersCached(mangaIntID)
	if err != nil || len(dbChapters) == 0 {
		return nil
	}
	out := make([]types.Chapter, len(dbChapters))
	for i, c := range dbChapters {
		out[i] = types.Chapter{
			ID:         c.SourceChapterID,
			MangaID:    fmt.Sprintf("%d", c.MangaID),
			Title:      c.Title,
			ChapterNum: c.ChapterNum,
			VolumeNum:  c.VolumeNum,
			ReleasedAt: c.FetchedAt,
		}
	}
	return out
}

// cachedMangaFallback returns cached data when plugin is unreachable.
func (s *AppService) cachedMangaFallback(pluginID, mangaID string, mangaIntID int64) (types.Manga, []types.Chapter, error) {
	cachedManga, err := s.db.GetMangaCached(pluginID, mangaID)
	if err != nil {
		return types.Manga{}, nil, fmt.Errorf("bridge: get manga detail: %w", err)
	}
	if cachedManga.ID == 0 {
		return types.Manga{}, nil, fmt.Errorf("bridge: plugin %s is unreachable and manga %s has no cached copy", pluginID, mangaID)
	}

	manga := types.Manga{
		ID:          cachedManga.SourceMangaID,
		Title:       cachedManga.Title,
		CoverURL:    cachedManga.CoverURL,
		Description: cachedManga.Description,
		Status:      cachedManga.Status,
	}

	if dbTitle, custom, err := s.db.MangaTitleIfCustom(pluginID, mangaID); err == nil && custom {
		manga.Title = dbTitle
	}
	if dbDesc, custom, err := s.db.MangaDescriptionIfCustom(pluginID, mangaID); err == nil && custom {
		manga.Description = dbDesc
	}
	if genres, has, err := s.GetMangaGenres(pluginID, mangaID); err == nil && has && len(genres) > 0 {
		manga.Genres = genres
	}
	if dim, has, _ := s.db.GetMangaCoverDim(mangaIntID); has && dim == 1 {
		manga.CoverDim = dim
	}
	dbChapters, err := s.db.ListChaptersCached(mangaIntID)
	if err != nil || len(dbChapters) == 0 {
		return manga, nil, nil
	}

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

// resolveChapterIntID looks up the integer chapter ID from source identifiers.
func (s *AppService) resolveChapterIntID(pluginID, mangaID, chapterID string) (int64, error) {
	return s.db.ResolveChapterIntID(pluginID, mangaID, chapterID)
}

// GetPageList delegates to the plugin's GetPageList function.
func (s *AppService) GetPageList(pluginID, chapterID string) ([]types.Page, error) {
	result, err := s.mgr.GetPageList(pluginID, chapterID)
	if err != nil {
		return nil, fmt.Errorf("bridge: get page list: %w", err)
	}
	if raw, merr := json.Marshal(result); merr == nil {
		chapterIntID, _ := s.resolveChapterIntID(pluginID, "", chapterID)
		if chapterIntID != 0 {
			if perr := s.db.SaveChapterPages(chapterIntID, raw); perr != nil {
				logger.Warn("cache chapter pages", "chapter", chapterID, "error", perr)
			}
		}
	}
	return result, nil
}

// GetPageListCached is like GetPageList but falls back to the local cache.
func (s *AppService) GetPageListCached(pluginID, chapterID string) ([]types.Page, error) {
	result, err := s.mgr.GetPageList(pluginID, chapterID)
	if err == nil {
		logger.Debug("page list cache: miss (online success)", "chapter", chapterID, "pages", len(result))
		if raw, merr := json.Marshal(result); merr == nil {
			chapterIntID, _ := s.resolveChapterIntID(pluginID, "", chapterID)
			if chapterIntID != 0 {
				if perr := s.db.SaveChapterPages(chapterIntID, raw); perr != nil {
					logger.Warn("cache chapter pages", "chapter", chapterID, "error", perr)
				}
			}
		}
		return result, nil
	}
	logger.Debug("page list cache: plugin failed, trying local cache", "chapter", chapterID, "plugin_err", err)
	chapterIntID, _ := s.resolveChapterIntID(pluginID, "", chapterID)
	if chapterIntID == 0 {
		return nil, fmt.Errorf("bridge: get page list: %w", err)
	}
	cached, cerr := s.db.GetChapterPages(chapterIntID)
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

// ToggleLibraryItem flips the in-library flag for a manga.
func (s *AppService) ToggleLibraryItem(pluginID, mangaID string) error {
	mangaIntID, _ := s.db.ResolveMangaIntID(pluginID, mangaID)
	if err := s.db.ToggleLibrary(mangaIntID); err != nil {
		return fmt.Errorf("bridge: toggle library item: %w", err)
	}
	return nil
}

// ListLibrary returns the user's in-library manga.
func (s *AppService) ListLibrary() ([]database.Manga, error) {
	list, err := s.db.ListLibrary()
	if err != nil {
		return nil, fmt.Errorf("bridge: list library: %w", err)
	}
	return list, nil
}

// SetMangaGenres stores a user-defined genre override for a manga.
func (s *AppService) SetMangaGenres(pluginID, mangaID string, genres []string) error {
	mangaIntID, _ := s.db.ResolveMangaIntID(pluginID, mangaID)
	if err := s.db.SetMangaGenres(mangaIntID, genres); err != nil {
		return fmt.Errorf("bridge: set manga genres: %w", err)
	}
	return nil
}

// GetMangaGenres returns the stored genre override for a manga.
func (s *AppService) GetMangaGenres(pluginID, mangaID string) ([]string, bool, error) {
	mangaIntID, _ := s.db.ResolveMangaIntID(pluginID, mangaID)
	genres, has, err := s.db.GetMangaGenres(mangaIntID)
	if err != nil {
		return nil, false, fmt.Errorf("bridge: get manga genres: %w", err)
	}
	return genres, has, nil
}

// AddGenre appends a genre to a manga's user-defined override.
func (s *AppService) AddGenre(pluginID, mangaID, genre string) error {
	mangaIntID, _ := s.db.ResolveMangaIntID(pluginID, mangaID)
	genres, has, err := s.db.GetMangaGenres(mangaIntID)
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
	if err := s.db.SetMangaGenres(mangaIntID, genres); err != nil {
		return fmt.Errorf("bridge: add genre: %w", err)
	}
	return nil
}

// ToggleGenre adds a genre to the override if not present, or removes it if present.
func (s *AppService) ToggleGenre(pluginID, mangaID, genre string) error {
	mangaIntID, _ := s.db.ResolveMangaIntID(pluginID, mangaID)
	genres, has, err := s.db.GetMangaGenres(mangaIntID)
	if err != nil {
		return fmt.Errorf("bridge: toggle genre: %w", err)
	}
	if !has {
		genres = nil
	}
	for i, g := range genres {
		if g == genre {
			genres = append(genres[:i], genres[i+1:]...)
			if len(genres) == 0 {
				genres = nil
			}
			if err := s.db.SetMangaGenres(mangaIntID, genres); err != nil {
				return fmt.Errorf("bridge: toggle genre: %w", err)
			}
			return nil
		}
	}
	genres = append(genres, genre)
	if err := s.db.SetMangaGenres(mangaIntID, genres); err != nil {
		return fmt.Errorf("bridge: toggle genre: %w", err)
	}
	return nil
}

// RemoveGenre removes one genre from a manga's user-defined override.
func (s *AppService) RemoveGenre(pluginID, mangaID, genre string) error {
	mangaIntID, _ := s.db.ResolveMangaIntID(pluginID, mangaID)
	genres, has, err := s.db.GetMangaGenres(mangaIntID)
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
	if err := s.db.SetMangaGenres(mangaIntID, filtered); err != nil {
		return fmt.Errorf("bridge: remove genre: %w", err)
	}
	return nil
}

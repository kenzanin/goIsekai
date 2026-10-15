package bridge

import (
	"context"
	"fmt"
	"slices"

	"github.com/goccy/go-json"
	"goisekai/internal/logger"
	"goisekai/pkg/types"
)

// SearchManga delegates to the plugin's Search function. Blank genre entries
// are dropped first: plugins read a non-empty genre list as "browse by genre",
// so the empty string a form submits for "all genres" would send them down
// that path with nothing to match. For offline plugins returns empty result.
func (s *AppService) SearchManga(pluginID string, filter types.SearchFilter) ([]types.Manga, error) {
	// Deactivated plugins must not issue remote requests.
	if !s.db.IsPluginActive(pluginID) {
		return nil, nil
	}

	filter.Genres = slices.DeleteFunc(filter.Genres, func(g string) bool { return g == "" })
	var (
		result []types.Manga
		err    error
	)
	runErr := s.runOnInteractive(context.Background(), pluginID, func(context.Context) error {
		result, err = s.mgr.Search(pluginID, filter)
		return err
	})
	if runErr != nil {
		return nil, fmt.Errorf("bridge: search manga: %w", runErr)
	}
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

// GetMangaDetailsCached returns cached manga/chapters immediately without
// blocking on the interactive lane. Use this for initial page render where
// stale data is acceptable; follow up with a background refresh if needed.
func (s *AppService) GetMangaDetailsCached(pluginID, mangaID string) (types.Manga, []types.Chapter, error) {
	mangaIntID, _ := s.db.ResolveMangaIntID(pluginID, mangaID)
	return s.cachedMangaFallback(pluginID, mangaID, mangaIntID)
}

// GetMangaDetails fetches a manga and its chapter list from a plugin.
// The plugin invoke rides the interactive lane (task 4.1); the body keeps
// calling s.mgr.* directly — routing s.GetChapterList from inside would nest
// same-lane awaits.
func (s *AppService) GetMangaDetails(pluginID, mangaID string) (types.Manga, []types.Chapter, error) {
	mangaIntID, _ := s.db.ResolveMangaIntID(pluginID, mangaID)

	// Deactivated plugins must not issue remote requests; their data stays readable from SQLite.
	if !s.db.IsPluginActive(pluginID) {
		return s.cachedMangaFallback(pluginID, mangaID, mangaIntID)
	}

	if s.mgr == nil {
		return s.cachedMangaFallback(pluginID, mangaID, mangaIntID)
	}

	var (
		manga types.Manga
		err   error
	)
	runErr := s.runOnInteractive(context.Background(), pluginID, func(context.Context) error {
		manga, err = s.mgr.GetMangaDetail(pluginID, mangaID)
		return err
	})
	if runErr != nil {
		return types.Manga{}, nil, fmt.Errorf("bridge: get manga details: %w", runErr)
	}
	if err == nil && manga.Title == "" {
		err = fmt.Errorf("empty title from plugin")
	}
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
			if dbCover, custom, err := s.db.MangaCoverIfCustom(pluginID, mangaID); err == nil && custom {
				manga.CoverURL = dbCover
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
		if dbCover, custom, err := s.db.MangaCoverIfCustom(pluginID, mangaID); err == nil && custom {
			manga.CoverURL = dbCover
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

// resolveChapterIntID looks up the integer chapter ID from source identifiers.
func (s *AppService) resolveChapterIntID(pluginID, mangaID, chapterID string) (int64, error) {
	return s.db.ResolveChapterIntID(pluginID, mangaID, chapterID)
}

// GetPageList delegates to the plugin's GetPageList function.
func (s *AppService) GetPageList(pluginID, chapterID string) ([]types.Page, error) {
	// Deactivated plugins must not issue remote requests.
	if !s.db.IsPluginActive(pluginID) {
		chapterIntID, _ := s.resolveChapterIntID(pluginID, "", chapterID)
		cached, err := s.db.GetChapterPages(chapterIntID)
		if err != nil || cached == nil {
			return nil, fmt.Errorf("bridge: get page list: plugin %s is offline and has no cached pages", pluginID)
		}
		var pages []types.Page
		if uerr := json.Unmarshal(cached, &pages); uerr != nil {
			return nil, fmt.Errorf("bridge: unmarshal cached pages: %w", uerr)
		}
		return pages, nil
	}

	var (
		result []types.Page
		err    error
	)
	runErr := s.runOnInteractive(context.Background(), pluginID, func(context.Context) error {
		result, err = s.mgr.GetPageList(pluginID, chapterID)
		return err
	})
	if runErr != nil {
		return nil, fmt.Errorf("bridge: get page list: %w", runErr)
	}
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

// GetPageListCached returns the cached page list from the database immediately
// without blocking on the interactive lane. Returns nil pages for empty result
// (plugin returned no pages) so caller can handle gracefully.
func (s *AppService) GetPageListCached(pluginID, chapterID string) ([]types.Page, error) {
	chapterIntID, _ := s.resolveChapterIntID(pluginID, "", chapterID)
	if chapterIntID == 0 {
		return nil, nil // chapter not found in DB - will fallback to live fetch
	}
	cached, err := s.db.GetChapterPages(chapterIntID)
	if err != nil {
		return nil, fmt.Errorf("bridge: get chapter pages: %w", err)
	}
	if cached == nil {
		return nil, nil // no cached pages - will fallback to live fetch
	}
	var pages []types.Page
	if uerr := json.Unmarshal(cached, &pages); uerr != nil {
		return nil, fmt.Errorf("bridge: unmarshal cached pages: %w", uerr)
	}
	return pages, nil
}

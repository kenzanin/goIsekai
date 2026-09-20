package bridge

import (
	"fmt"
	"slices"

	"goisekai/internal/database"
	"goisekai/internal/logger"
	"goisekai/pkg/types"

	"github.com/goccy/go-json"
)

// SearchManga delegates to the plugin's Search function. Blank genre entries
// are dropped first: plugins read a non-empty genre list as "browse by genre",
// so the empty string a form submits for "all genres" would send them down
// that path with nothing to match.
func (s *AppService) SearchManga(pluginID string, filter types.SearchFilter) ([]types.Manga, error) {
	filter.Genres = slices.DeleteFunc(filter.Genres, func(g string) bool { return g == "" })
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

package bridge

import (
	"context"
	"fmt"
	"slices"

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

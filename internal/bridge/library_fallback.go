package bridge

import (
	"fmt"

	"goisekai/pkg/types"
)

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

// CachedMangaAndChapters returns the persisted copy of a manga and its chapter
// list (newest first). The reader prefers this over a live plugin fetch: the
// live chapter list can come back partial, which leaves chapter navigation with
// no next chapter even though the detail page lists one.
func (s *AppService) CachedMangaAndChapters(pluginID, mangaID string) (types.Manga, []types.Chapter, error) {
	mangaIntID, _ := s.db.ResolveMangaIntID(pluginID, mangaID)
	return s.cachedMangaFallback(pluginID, mangaID, mangaIntID)
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

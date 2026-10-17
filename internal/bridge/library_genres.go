package bridge

import (
	"fmt"
)

// displayedGenres returns the genres the detail page currently shows: the
// user override when one exists, otherwise the enrichment categories (the
// persisted mirror of plugin genres shown in the panel). Genre toggles seed
// from this list so adding one genre never discards the rest.
func (s *AppService) displayedGenres(mangaIntID int64) []string {
	if genres, has, err := s.db.GetMangaGenres(mangaIntID); err == nil && has {
		return genres
	}
	cats, _ := s.db.ListCategories(fmt.Sprint(mangaIntID))
	names := make([]string, 0, len(cats))
	for _, c := range cats {
		names = append(names, c.Category)
	}
	return names
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

// ToggleGenre adds a genre to the override if not present, or removes it if present.
func (s *AppService) ToggleGenre(pluginID, mangaID, genre string) error {
	mangaIntID, _ := s.db.ResolveMangaIntID(pluginID, mangaID)
	genres, has, err := s.db.GetMangaGenres(mangaIntID)
	if err != nil {
		return fmt.Errorf("bridge: toggle genre: %w", err)
	}
	if !has {
		genres = s.displayedGenres(mangaIntID)
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

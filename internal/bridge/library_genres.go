package bridge

import (
	"fmt"
	"slices"
)

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

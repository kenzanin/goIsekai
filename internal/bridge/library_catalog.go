package bridge

import (
	"fmt"

	"goisekai/internal/database"
)

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

// ListLibraryCategories returns in-library categories keyed by manga row-ID.
func (s *AppService) ListLibraryCategories() (map[string][]string, error) {
	m, err := s.db.LibraryCategories()
	if err != nil {
		return nil, fmt.Errorf("bridge: library categories: %w", err)
	}
	return m, nil
}

// LibraryCategoryCounts returns how many in-library manga carry each category.
func (s *AppService) LibraryCategoryCounts() (map[string]int, error) {
	c, err := s.db.LibraryCategoryCounts()
	if err != nil {
		return nil, fmt.Errorf("bridge: library category counts: %w", err)
	}
	return c, nil
}

// LibraryCategoryCountsOrEmpty is the filter's tag list; a failed query must not
// take the library page down with it.
func (s *AppService) LibraryCategoryCountsOrEmpty() map[string]int {
	c, err := s.LibraryCategoryCounts()
	if err != nil {
		return map[string]int{}
	}
	return c
}

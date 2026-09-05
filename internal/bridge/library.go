package bridge

import (
	"fmt"

	"goisekai/internal/database"
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

// GetPageList delegates to the plugin's GetPageList function.
func (s *AppService) GetPageList(pluginID, chapterID string) ([]types.Page, error) {
	result, err := s.mgr.GetPageList(pluginID, chapterID)
	if err != nil {
		return nil, fmt.Errorf("bridge: get page list: %w", err)
	}
	return result, nil
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

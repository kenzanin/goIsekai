package bridge

import (
	"encoding/json"
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

// FetchAltTitles resolves alternative titles via the alt-titles provider
// plugin (capability-discovered, never hardcoded) and persists them on the
// manga. Returns the stored payload {source, titles}.
func (s *AppService) FetchAltTitles(pluginID, mangaID string) (map[string]any, error) {
	manga, _, err := s.GetMangaDetails(pluginID, mangaID)
	if err != nil {
		return nil, err
	}
	res, err := s.mgr.GetAltTitles(manga.Title)
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(res)
	if err != nil {
		return nil, err
	}
	if err := s.db.SaveAltTitles(pluginID, mangaID, string(payload)); err != nil {
		return nil, err
	}
	out := map[string]any{"source": res.Source, "titles": res.Titles}
	if out["titles"] == nil {
		out["titles"] = []string{}
	}
	return out, nil
}

// StoredAltTitles returns the persisted alt-titles payload for a manga, or nil.
func (s *AppService) StoredAltTitles(pluginID, mangaID string) map[string]any {
	raw, err := s.db.GetAltTitles(pluginID, mangaID)
	if err != nil || raw == "" {
		return nil
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil
	}
	if out["titles"] == nil {
		out["titles"] = []string{}
	}
	return out
}

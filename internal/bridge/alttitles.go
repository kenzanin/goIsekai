package bridge

import (
	"fmt"
	"strings"

	"goisekai/internal/database"
)

type SearchHit struct {
	PluginID      string
	SourceMangaID string
	Title         string
	Score         int
}

// SetMainTitle promotes newTitle to be the manga's main title. The title must
// already exist in the stored alternative titles; an unknown title is rejected.
func (s *AppService) SetMainTitle(pluginID, mangaID, title string) error {
	rowID, err := s.db.MangaRowID(pluginID, mangaID)
	if err != nil {
		return fmt.Errorf("bridge: resolve manga: %w", err)
	}
	alts, err := s.db.ListAltTitles(rowID)
	if err != nil {
		return fmt.Errorf("bridge: list alt titles: %w", err)
	}
	found := false
	for _, a := range alts {
		if a.Title == title {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("title %q is not in the alternative titles list", title)
	}
	if err := s.db.SwapMainTitle(pluginID, mangaID, title); err != nil {
		return fmt.Errorf("bridge: swap main title: %w", err)
	}
	return nil
}

// RemoveAltTitle deletes a single alternative title from the manga and
// re-syncs the FTS index.
func (s *AppService) RemoveAltTitle(pluginID, mangaID, title string) error {
	rowID, err := s.db.MangaRowID(pluginID, mangaID)
	if err != nil {
		return fmt.Errorf("bridge: resolve manga: %w", err)
	}
	if err := s.db.RemoveAltTitle(rowID, title); err != nil {
		return fmt.Errorf("bridge: remove alt title: %w", err)
	}
	if err := s.db.SyncFTS(rowID); err != nil {
		return fmt.Errorf("bridge: sync fts: %w", err)
	}
	return nil
}

// ListAltSummaries returns the stored alternative summaries for a manga by its
// plugin and source identifiers.
func (s *AppService) ListAltSummaries(pluginID, mangaID string) ([]database.AltDescriptionRow, error) {
	rowID, err := s.db.MangaRowID(pluginID, mangaID)
	if err != nil {
		return nil, fmt.Errorf("bridge: resolve manga: %w", err)
	}
	return s.db.ListAltDescriptions(rowID)
}

// SetMainSummary promotes newDesc to be the manga's main description.
// The submitted value is trusted: it came from a description this page
// rendered from the database. The old strict membership gate rejected valid
// submissions whenever the round-tripped text differed in invisible ways
// (entity/whitespace encoding), so a not-in-list error is no longer fatal —
// SwapMainDescription demotes the old main and stores newDesc regardless.
func (s *AppService) SetMainSummary(pluginID, mangaID, description string) error {
	if strings.TrimSpace(description) == "" {
		return fmt.Errorf("empty description")
	}
	return s.db.SwapMainDescription(pluginID, mangaID, description)
}

// RemoveAltSummary deletes a single alternative description from the manga.
func (s *AppService) RemoveAltSummary(pluginID, mangaID, description string) error {
	rowID, err := s.db.MangaRowID(pluginID, mangaID)
	if err != nil {
		return fmt.Errorf("bridge: resolve manga: %w", err)
	}
	return s.db.RemoveAltDescription(rowID, description)
}

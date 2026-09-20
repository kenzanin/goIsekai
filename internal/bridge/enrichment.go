package bridge

import (
	"fmt"

	"goisekai/internal/database"
	"goisekai/internal/enrich"
)

// EnrichmentResult holds all enrichment data for a manga.
type EnrichmentResult struct {
	AltTitles    []database.EnrichmentRow
	AltSummaries []database.EnrichmentRow
	Categories   []database.EnrichmentRow
	Related      []database.EnrichmentRow
}

// EnrichmentCatalogEntry describes one enrichment source for the UI.
type EnrichmentCatalogEntry struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Kinds      []string `json:"kinds"`
	Precedence int      `json:"precedence,omitempty"`
	Enabled    bool     `json:"enabled"`
}

// loadInfoProviders instantiates the enrichment scripts so the providers they
// declare are registered before the catalog or a fetch reads it.
func (s *AppService) loadInfoProviders() {
	if s.mgr != nil {
		s.mgr.LoadEnrichmentProviders()
	}
}

// EnrichmentSources returns all registered enrichment source IDs in provider
// registration order, which is the precedence order a fetch walks: the first
// source that answers a kind owns it and later sources only fill the gaps.
func (s *AppService) EnrichmentSources() []string {
	if s.enrich == nil {
		return nil
	}
	s.loadInfoProviders()
	entries := s.enrich.Catalog("")
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = e.ID
	}
	return out
}

// EnrichmentCatalog returns all registered enrichment sources (built-in +
// plugin-declared). The caller passes an optional kind filter; empty string
// returns all.
func (s *AppService) EnrichmentCatalog(kind string) []EnrichmentCatalogEntry {
	if s.enrich == nil {
		return nil
	}
	s.loadInfoProviders()
	entries := s.enrich.Catalog(enrich.Kind(kind))
	out := make([]EnrichmentCatalogEntry, 0, len(entries))
	for _, e := range entries {
		kinds := make([]string, len(e.Kinds))
		for i, k := range e.Kinds {
			kinds[i] = string(k)
		}
		out = append(out, EnrichmentCatalogEntry{
			ID:         e.ID,
			Name:       e.Name,
			Kinds:      kinds,
			Precedence: int(e.Precedence),
			Enabled:    e.Enabled,
		})
	}
	return out
}

// RemoveCategory deletes a single category from a manga's stored enrichment data.
func (s *AppService) RemoveCategory(pluginID, mangaID, category string) error {
	rowID, err := s.db.ResolveMangaRowID(pluginID, mangaID)
	if err != nil {
		return err
	}
	return s.db.RemoveCategory(rowID, category)
}

// AddCategory adds a category to a manga's stored enrichment data.
func (s *AppService) AddCategory(pluginID, mangaID, category string) error {
	rowID, err := s.db.ResolveMangaRowID(pluginID, mangaID)
	if err != nil {
		return err
	}
	return s.db.AddCategory(rowID, category)
}

// RemoveRelated deletes a single related/recommended manga from storage.
func (s *AppService) RemoveRelated(pluginID, mangaID, title string) error {
	rowID, err := s.db.ResolveMangaRowID(pluginID, mangaID)
	if err != nil {
		return err
	}
	return s.db.RemoveRelated(rowID, title)
}

// GetEnrichment returns all enrichment data for a manga from the database.
func (s *AppService) GetEnrichment(pluginID, mangaID string) (*EnrichmentResult, error) {
	rowID, err := s.db.ResolveMangaRowID(pluginID, mangaID)
	if err != nil {
		return nil, err
	}

	res := &EnrichmentResult{}

	// alt_titles and alt_summaries are tables, not enrichment kinds, so they
	// come through their own listers and are reshaped into the shared row.
	titles, err := s.db.ListAltTitles(rowID)
	if err != nil {
		return nil, err
	}
	res.AltTitles = make([]database.EnrichmentRow, len(titles))
	for i, t := range titles {
		res.AltTitles[i] = database.EnrichmentRow{Value: t.Title, Source: t.Source}
	}
	summaries, err := s.db.ListAltDescriptions(rowID)
	if err != nil {
		return nil, err
	}
	res.AltSummaries = make([]database.EnrichmentRow, len(summaries))
	for i, a := range summaries {
		res.AltSummaries[i] = database.EnrichmentRow{Value: a.Description, Source: a.Source}
	}

	res.Categories, err = s.db.ListEnrichment(rowID, string(enrich.KindCategories))
	if err != nil {
		return nil, err
	}
	res.Related, err = s.db.ListEnrichment(rowID, string(enrich.KindRelated))
	if err != nil {
		return nil, err
	}
	return res, nil
}

// ResetEnrichment deletes all user enrichment data (alt titles, summaries,
// categories, related) and restores title/synopsis/genres to original plugin values.
func (s *AppService) ResetEnrichment(pluginID, mangaID string) error {
	rowID, err := s.db.ResolveMangaRowID(pluginID, mangaID)
	if err != nil {
		return err
	}
	if err := s.db.ResetEnrichment(rowID); err != nil {
		return fmt.Errorf("reset enrichment: %w", err)
	}
	mangaIntID, _ := s.db.ResolveMangaIntID(pluginID, mangaID)
	if err := s.db.SetMangaGenres(mangaIntID, nil); err != nil {
		return fmt.Errorf("reset genres: %w", err)
	}
	mangaIntID, _ = s.db.ResolveMangaIntID(pluginID, mangaID)
	if err := s.db.SetMangaAuthor(mangaIntID, ""); err != nil {
		return fmt.Errorf("reset author: %w", err)
	}
	return nil
}

// StoredAuthor returns the author captured by an enrichment provider.
// ok is false when nothing was fetched yet.
func (s *AppService) StoredAuthor(pluginID, mangaID string) (author string, ok bool) {
	mangaIntID, _ := s.db.ResolveMangaIntID(pluginID, mangaID)
	author, ok, err := s.db.GetMangaAuthor(mangaIntID)
	if err != nil {
		return "", false
	}
	return author, ok
}

// ListCategories returns enrichment categories for a manga from the database.
func (s *AppService) ListCategories(pluginID, mangaID string) ([]database.EnrichmentRow, error) {
	rowID, err := s.db.ResolveMangaRowID(pluginID, mangaID)
	if err != nil {
		return nil, err
	}
	return s.db.ListEnrichment(rowID, "categories")
}

// ListRelated returns enrichment related/recommended manga for a manga from the database.
func (s *AppService) ListRelated(pluginID, mangaID string) ([]database.EnrichmentRow, error) {
	rowID, err := s.db.ResolveMangaRowID(pluginID, mangaID)
	if err != nil {
		return nil, err
	}
	return s.db.ListEnrichment(rowID, "related")
}

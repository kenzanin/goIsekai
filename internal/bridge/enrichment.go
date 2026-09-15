package bridge

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"goisekai/internal/database"
	"goisekai/internal/enrich"
	"goisekai/internal/logger"
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
	ID    string   `json:"id"`
	Name  string   `json:"name"`
	Kinds []string `json:"kinds"`
}

// loadInfoProviders instantiates the enrichment scripts so the providers they
// declare are registered before the catalog or a fetch reads it.
func (s *AppService) loadInfoProviders() {
	if s.mgr != nil {
		s.mgr.LoadEnrichmentProviders()
	}
}

// FetchEnrichment fetches enrichment data from external sources and stores it.
func (s *AppService) FetchEnrichment(pluginID, mangaID, title string, sources []string) error {
	if s.enrich == nil {
		return fmt.Errorf("enrichment provider not configured")
	}
	s.loadInfoProviders()
	rowID, err := s.db.ResolveMangaRowID(pluginID, mangaID)
	if err != nil {
		return err
	}
	if strings.TrimSpace(title) == "" {
		// Providers answer an empty search with arbitrary trending results,
		// which would then be stored as this manga's enrichment data. Fall
		// back to the manga's own title instead of searching for nothing.
		title, err = s.db.MangaTitle(pluginID, mangaID)
		if err != nil {
			return fmt.Errorf("bridge: resolve title: %w", err)
		}
		if strings.TrimSpace(title) == "" {
			return fmt.Errorf("enrichment requires a title")
		}
	}
	logger.Debug("enrich fetch start", "title", title, "sources", sources)
	items := s.enrich.FetchFirst(context.Background(), &http.Client{}, title, sources)

	// Store alt titles.
	if titles, ok := items[enrich.KindTitles]; ok && len(titles) > 0 {
		names := make([]string, len(titles))
		for i, t := range titles {
			names[i] = t.Value
		}
		n, err := s.db.AddAltTitles(rowID, names, titles[0].Source)
		if err != nil {
			logger.Warn("store titles", "error", err)
		} else {
			logger.Info("enrich titles stored", "count", len(titles), "inserted", n, "source", titles[0].Source)
		}
	} else {
		logger.Debug("enrich titles: none found")
	}

	// Store alt summaries.
	if summs, ok := items[enrich.KindSummaries]; ok && len(summs) > 0 {
		names := make([]string, len(summs))
		for i, s := range summs {
			names[i] = s.Value
		}
		n, err := s.db.AddAltDescriptions(rowID, names, summs[0].Source)
		if err != nil {
			logger.Warn("store summaries", "error", err)
		} else {
			logger.Info("enrich summaries stored", "count", len(summs), "inserted", n, "source", summs[0].Source)
		}
	} else {
		logger.Debug("enrich summaries: none found")
	}

	// Store categories.
	if cats, ok := items[enrich.KindCategories]; ok && len(cats) > 0 {
		names := make([]string, len(cats))
		for i, c := range cats {
			names[i] = c.Value
		}
		n, err := s.db.AddCategories(rowID, names, cats[0].Source)
		if err != nil {
			logger.Warn("store categories", "error", err)
		} else {
			logger.Info("enrich categories stored", "count", len(cats), "inserted", n, "source", cats[0].Source)
		}
	} else {
		logger.Debug("enrich categories: none found")
	}
	// Store author (a single value, so the provider's items are joined).
	if authors, ok := items[enrich.KindAuthors]; ok && len(authors) > 0 {
		names := make([]string, 0, len(authors))
		for _, a := range authors {
			if strings.TrimSpace(a.Value) != "" {
				names = append(names, strings.TrimSpace(a.Value))
			}
		}
		if len(names) > 0 {
			if err := s.db.SetMangaAuthor(rowID, strings.Join(names, ", ")); err != nil {
				logger.Warn("store author", "error", err)
			} else {
				logger.Info("enrich author stored", "author", strings.Join(names, ", "), "source", authors[0].Source)
			}
		}
	} else {
		logger.Debug("enrich authors: none found")
	}

	// Store related manga.
	if rels, ok := items[enrich.KindRelated]; ok && len(rels) > 0 {
		rows := make([]database.RelatedRow, len(rels))
		for i, r := range rels {
			rows[i] = database.RelatedRow{Title: r.Value, URL: r.URL, Source: r.Source}
		}
		n, err := s.db.AddRelated(rowID, rows, rels[0].Source)
		if err != nil {
			logger.Warn("store related", "error", err)
		} else {
			logger.Info("enrich related stored", "count", len(rels), "inserted", n, "source", rels[0].Source)
		}
	} else {
		logger.Debug("enrich related: none found")
	}
	return nil
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
			ID:    e.ID,
			Name:  e.Name,
			Kinds: kinds,
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
	if err := s.db.SetMangaGenres(rowID, nil); err != nil {
		return fmt.Errorf("reset genres: %w", err)
	}
	if err := s.db.SetMangaAuthor(rowID, ""); err != nil {
		return fmt.Errorf("reset author: %w", err)
	}
	return nil
}

// StoredAuthor returns the author captured by an enrichment provider.
// ok is false when nothing was fetched yet.
func (s *AppService) StoredAuthor(pluginID, mangaID string) (author string, ok bool) {
	rowID, err := s.db.ResolveMangaRowID(pluginID, mangaID)
	if err != nil {
		return "", false
	}
	author, ok, err = s.db.GetMangaAuthor(rowID)
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

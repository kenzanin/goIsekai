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
			mangaIntID, _ := s.db.ResolveMangaIntID(pluginID, mangaID)
			if err := s.db.SetMangaAuthor(mangaIntID, strings.Join(names, ", ")); err != nil {
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

package bridge

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"goisekai/internal/enrich"
	"goisekai/internal/logger"
)

// FetchEnrichment fetches enrichment data from external sources and stores it.
// When sources is empty, fetches from all enabled providers; when non-empty,
// fetches only from those sources (backward-compatible single-source mode).
func (s *AppService) FetchEnrichment(pluginID, mangaID, title string, sources []string) error {
	// task 3.1: ride the fetch lane so a slow source serializes only its own
	// enrichment work, not the whole request path.
	return s.runOnFetch(context.Background(), pluginID, func(context.Context) error {
		return s.fetchEnrichment(pluginID, mangaID, title, sources)
	})
}

func (s *AppService) fetchEnrichment(pluginID, mangaID, title string, sources []string) error {
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

	var items map[enrich.Kind][]enrich.Item

	// Multi-source mode: fetch from all enabled providers and merge. A kind
	// stays first-source-wins only when the request names sources explicitly
	// (the per-source API), so the UI button fills gaps from every source.
	if len(sources) == 0 {
		items = s.enrich.FetchAll(context.Background(), &http.Client{}, title, []enrich.Kind{
			enrich.KindTitles, enrich.KindSummaries,
			enrich.KindCategories, enrich.KindRelated, enrich.KindAuthors,
			enrich.KindCovers,
		})
	} else if len(sources) == 1 {
		items = s.enrich.FetchFirst(context.Background(), &http.Client{}, title, sources)
	} else {
		merged := make(map[enrich.Kind][]enrich.Item)
		for _, source := range sources {
			for kind, kindItems := range s.enrich.FetchFirst(context.Background(), &http.Client{}, title, []string{source}) {
				merged[kind] = append(merged[kind], kindItems...)
			}
		}
		items = merged
	}

	// Store items grouped by source. For authors, only store the highest-precedence non-blank result.
	s.storeEnrichment(pluginID, mangaID, rowID, items, s.enrich.OrderedProviders())
	return nil
}

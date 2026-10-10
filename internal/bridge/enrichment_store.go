package bridge

import (
	"strings"

	"goisekai/internal/database"
	"goisekai/internal/enrich"
	"goisekai/internal/logger"
)

// storeEnrichment stores fetched items by source. For items that support multiple sources
// (categories, titles, etc.), each source's items are stored separately. For authors,
// only the highest-precedence non-blank author is stored.
func (s *AppService) storeEnrichment(pluginID, mangaID, rowID string, items map[enrich.Kind][]enrich.Item, providers []enrich.Provider) {
	// Group items by source for multi-source storage
	sourceItems := make(map[string]map[enrich.Kind][]enrich.Item)
	for kind, kindItems := range items {
		for _, item := range kindItems {
			src := item.Source
			if src == "" {
				src = "unknown"
			}
			if sourceItems[src] == nil {
				sourceItems[src] = make(map[enrich.Kind][]enrich.Item)
			}
			sourceItems[src][kind] = append(sourceItems[src][kind], item)
		}
	}

	// Store alt titles per source
	if titles, ok := items[enrich.KindTitles]; ok && len(titles) > 0 {
		for src, srcItems := range sourceItems {
			if srcTitles, ok := srcItems[enrich.KindTitles]; ok && len(srcTitles) > 0 {
				names := make([]string, len(srcTitles))
				for i, t := range srcTitles {
					names[i] = t.Value
				}
				n, err := s.db.AddAltTitles(rowID, names, src)
				if err != nil {
					logger.Warn("store titles", "error", err)
				} else {
					logger.Info("enrich titles stored", "count", len(srcTitles), "inserted", n, "source", src)
				}
			}
		}
	} else {
		logger.Debug("enrich titles: none found")
	}

	// Store alt summaries per source
	if summs, ok := items[enrich.KindSummaries]; ok && len(summs) > 0 {
		for src, srcItems := range sourceItems {
			if srcSummaries, ok := srcItems[enrich.KindSummaries]; ok && len(srcSummaries) > 0 {
				names := make([]string, len(srcSummaries))
				for i, s := range srcSummaries {
					names[i] = s.Value
				}
				n, err := s.db.AddAltDescriptions(rowID, names, src)
				if err != nil {
					logger.Warn("store summaries", "error", err)
				} else {
					logger.Info("enrich summaries stored", "count", len(srcSummaries), "inserted", n, "source", src)
				}
			}
		}
	} else {
		logger.Debug("enrich summaries: none found")
	}

	// Store categories per source
	if cats, ok := items[enrich.KindCategories]; ok && len(cats) > 0 {
		for src, srcItems := range sourceItems {
			if srcCats, ok := srcItems[enrich.KindCategories]; ok && len(srcCats) > 0 {
				// Normalize categories through genre alias index before storing
				rawNames := make([]string, len(srcCats))
				for i, c := range srcCats {
					rawNames[i] = c.Value
				}
				names := s.genres.normalize(rawNames)
				if len(names) > 0 {
					n, err := s.db.AddCategories(rowID, names, src)
					if err != nil {
						logger.Warn("store categories", "error", err)
					} else {
						logger.Info("enrich categories stored", "count", len(srcCats), "inserted", n, "source", src)
					}
				}
			}
		}
	} else {
		logger.Debug("enrich categories: none found")
	}

	// Store author - only highest-precedence non-blank author
	if authors, ok := items[enrich.KindAuthors]; ok && len(authors) > 0 {
		// Find highest-precedence source with non-blank author
		for _, p := range providers {
			srcAuthors := sourceItems[p.ID()]
			if srcAuthors == nil {
				continue
			}
			kindAuthors, ok := srcAuthors[enrich.KindAuthors]
			if !ok || len(kindAuthors) == 0 {
				continue
			}
			// Find first non-blank author
			for _, a := range kindAuthors {
				if strings.TrimSpace(a.Value) != "" {
					mangaIntID, _ := s.db.ResolveMangaIntID(pluginID, mangaID)
					if err := s.db.SetMangaAuthor(mangaIntID, a.Value); err != nil {
						logger.Warn("store author", "error", err)
					} else {
						logger.Info("enrich author stored", "author", a.Value, "source", p.ID())
					}
					goto authorStored
				}
			}
		}
	authorStored:
	} else {
		logger.Debug("enrich authors: none found")
	}

	// Store alternative cover candidates per source.
	if covers, ok := items[enrich.KindCovers]; ok && len(covers) > 0 {
		rows := make([]database.AltCoverRow, 0, len(covers))
		for _, c := range covers {
			rows = append(rows, database.AltCoverRow{URL: c.Value, Source: c.Source})
		}
		n, err := s.db.AddAltCovers(rowID, rows)
		if err != nil {
			logger.Warn("store alt covers", "error", err)
		} else {
			logger.Info("enrich covers stored", "count", len(covers), "inserted", n)
		}
	}

	// Store related manga per source
	if rels, ok := items[enrich.KindRelated]; ok && len(rels) > 0 {
		for src, srcItems := range sourceItems {
			if srcRels, ok := srcItems[enrich.KindRelated]; ok && len(srcRels) > 0 {
				rows := make([]database.RelatedRow, len(srcRels))
				for i, r := range srcRels {
					rows[i] = database.RelatedRow{Title: r.Value, URL: r.URL, Source: r.Source}
				}
				n, err := s.db.AddRelated(rowID, rows, src)
				if err != nil {
					logger.Warn("store related", "error", err)
				} else {
					logger.Info("enrich related stored", "count", len(srcRels), "inserted", n, "source", src)
				}
			}
		}
	} else {
		logger.Debug("enrich related: none found")
	}
}

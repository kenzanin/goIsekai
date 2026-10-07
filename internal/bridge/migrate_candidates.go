package bridge

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"goisekai/internal/logger"
	"goisekai/internal/pluginutil"
	"goisekai/pkg/types"
)

// MigrationCandidate is one result from searching another source for the same title.
type MigrationCandidate struct {
	PluginID      string
	PluginName    string
	SourceMangaID string
	Title         string
	IsExactMatch  bool
}

// CollectMigrationCandidates searches every active source (except the entry's own)
// for the entry's display title. Every returned result is a candidate; an empty
// result set with nil error is not a failure. A per-plugin error skips that
// source; only when every searched source fails does the method return an
// error (sources unreachable, distinct from "no candidates").
func (s *AppService) CollectMigrationCandidates(pluginID, mangaID string) ([]MigrationCandidate, []string, error) {
	// title resolution happens below
	// Resolve display title: prefer DB title, fall back to cached manga title.
	displayTitle := ""
	if m, err := s.db.GetMangaCached(pluginID, mangaID); err == nil && m.Title != "" {
		displayTitle = m.Title
	}
	if displayTitle == "" {
		// Try live detail as last resort for title.
		if s.mgr != nil {
			if live, lerr := s.mgr.GetMangaDetail(pluginID, mangaID); lerr == nil && live.Title != "" {
				displayTitle = live.Title
			}
		}
	}
	if strings.TrimSpace(displayTitle) == "" {
		return nil, nil, fmt.Errorf("migration: cannot determine title for %s/%s", pluginID, mangaID)
	}
	return s.collectCandidatesWithTitle(pluginID, displayTitle, s.altTitlesFor(pluginID, mangaID))
}

// altTitlesFor returns the entry's alternative titles. They come from the
// enrichment sources (mangaupdates, mangadex), so they are alternate names for the
// same work rather than loose keywords, which makes them a trustworthy match key.
// A failure here is not an error: candidates are still collected on the primary
// title alone.
func (s *AppService) altTitlesFor(pluginID, mangaID string) []string {
	rowID, err := s.db.ResolveMangaIntID(pluginID, mangaID)
	if err != nil || rowID == 0 {
		return nil
	}
	rows, err := s.db.ListAltTitles(strconv.FormatInt(rowID, 10))
	if err != nil {
		logger.Warn("migration alt titles unavailable", "plugin", pluginID, "manga", mangaID, "error", err)
		return nil
	}
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		if strings.TrimSpace(r.Title) != "" {
			out = append(out, r.Title)
		}
	}
	return out
}

// matchKeys builds the set of normalized titles a candidate may equal to count as
// an exact match: the entry's display title plus every alternative title. Empty
// keys are never added, otherwise a candidate with a blank title would match one
// and get auto-selected.
func matchKeys(title string, altTitles []string) map[string]struct{} {
	keys := make(map[string]struct{}, len(altTitles)+1)
	if k := pluginutil.NormalizeTitle(title); k != "" {
		keys[k] = struct{}{}
	}
	for _, a := range altTitles {
		if k := pluginutil.NormalizeTitle(a); k != "" {
			keys[k] = struct{}{}
		}
	}
	return keys
}

func isExactMatch(candidateTitle string, keys map[string]struct{}) bool {
	k := pluginutil.NormalizeTitle(candidateTitle)
	if k == "" {
		return false
	}
	_, ok := keys[k]
	return ok
}

// collectCandidatesWithTitle searches every source for the entry's title. A
// candidate is an exact match when its title equals the entry's display title or
// any of its alternative titles, so a source that publishes the series under a
// different name is still recognised as the same work.
func (s *AppService) collectCandidatesWithTitle(currentPluginID, title string, altTitles []string) ([]MigrationCandidate, []string, error) {
	keys := matchKeys(title, altTitles)
	var candidates []MigrationCandidate
	var failures []string
	searched := 0
	if s.mgr == nil {
		return nil, nil, fmt.Errorf("migration: plugin manager not available")
	}
	// Active map from DB (is_active only written on insert, DB is source of truth).
	activeMap := make(map[string]bool)
	if dbPlugins, err := s.db.ListPlugins(); err == nil {
		for _, dp := range dbPlugins {
			activeMap[dp.ID] = dp.IsActive
		}
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	for _, p := range s.mgr.LoadedPlugins() {
		if p.ID == currentPluginID {
			continue
		}
		if active, ok := activeMap[p.ID]; ok && !active {
			continue
		}
		searched++
		wg.Add(1)
		go func(plugID, plugName string) {
			defer wg.Done()
			// task 3.1: each source's search rides the fetch lane under its own
			// PluginKey, so one slow source serializes only its own lookups while
			// the other sources keep going.
			searchErr := s.runOnFetch(context.Background(), plugID, func(context.Context) error {
				results, err := s.mgr.Search(plugID, types.SearchFilter{Query: title})
				if err != nil {
					return err
				}
				if len(results) == 0 {
					return nil
				}
				local := make([]MigrationCandidate, 0, len(results))
				for _, r := range results {
					local = append(local, MigrationCandidate{
						PluginID:      plugID,
						PluginName:    plugName,
						SourceMangaID: r.ID,
						Title:         r.Title,
						IsExactMatch:  isExactMatch(r.Title, keys),
					})
				}
				mu.Lock()
				candidates = append(candidates, local...)
				mu.Unlock()
				return nil
			})
			if searchErr != nil {
				logger.Warn("migration candidate search failed", "plugin", plugID, "error", searchErr)
				mu.Lock()
				failures = append(failures, plugID)
				mu.Unlock()
			}
		}(p.ID, p.Name)
	}
	wg.Wait()
	if searched > 0 && len(candidates) == 0 && len(failures) == searched {
		return nil, failures, fmt.Errorf("migration: sources could not be reached")
	}
	return candidates, failures, nil
}

// AutoSelectCandidate returns the single exact match when there is exactly one,
// otherwise nil (caller must present the list).
func AutoSelectCandidate(candidates []MigrationCandidate) *MigrationCandidate {
	var exact *MigrationCandidate
	count := 0
	for i := range candidates {
		if candidates[i].IsExactMatch {
			count++
			if exact == nil {
				exact = &candidates[i]
			}
		}
	}
	if count == 1 {
		return exact
	}
	return nil
}

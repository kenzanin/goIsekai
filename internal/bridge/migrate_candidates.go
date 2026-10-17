package bridge

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"goisekai/internal/logger"
	"goisekai/internal/pluginutil"
	"goisekai/internal/workers"
	"goisekai/pkg/types"
)

// MigrationCandidate is one result from searching another source for the same title.
type MigrationCandidate struct {
	PluginID      string
	PluginName    string
	SourceMangaID string
	Title         string
	CoverURL      string
	IsExactMatch  bool
}

// CollectMigrationCandidates searches each target source for query and returns
// every result as a candidate. An empty result set with a nil error is not a
// failure; a per-plugin error goes into failures and skips that source.
//
// An empty query searches nothing — the migration picker shows an empty page
// rather than hammering every source on a page the user has not searched yet.
//
// A candidate counts as an exact match when its title equals the query or any of
// the entry's alternative titles, so a source that publishes the series under a
// different name is still recognised as the same work.
func (s *AppService) CollectMigrationCandidates(pluginID, mangaID string, targets []string, query string) ([]MigrationCandidate, []string, error) {
	if strings.TrimSpace(query) == "" {
		return nil, nil, nil
	}
	if s.mgr == nil {
		return nil, nil, fmt.Errorf("migration: plugin manager not available")
	}
	keys := matchKeys(query, s.altTitlesFor(pluginID, mangaID))

	names := make(map[string]string)
	for _, p := range s.mgr.LoadedPlugins() {
		if p.Name != "" {
			names[p.ID] = p.Name
			continue
		}
		names[p.ID] = p.ID
	}

	var mu sync.Mutex
	var candidates []MigrationCandidate
	var failures []string
	futures := make(map[string]*workers.Future)
	for _, id := range targets {
		// Each source's search rides the fetch lane under its own PluginKey, so
		// one slow source serializes only its own lookups while the other
		// sources keep going.
		fut, err := s.EnqueueSearchManga(context.Background(), id, types.SearchFilter{Query: query, Page: 1}, func(_ context.Context, results []types.Manga, err error) {
			if err != nil {
				logger.Warn("migration candidate search failed", "plugin", id, "error", err)
				mu.Lock()
				failures = append(failures, id)
				mu.Unlock()
				return
			}
			if len(results) == 0 {
				return
			}
			local := make([]MigrationCandidate, 0, len(results))
			for _, r := range results {
				local = append(local, MigrationCandidate{
					PluginID:      id,
					PluginName:    names[id],
					SourceMangaID: r.ID,
					Title:         r.Title,
					CoverURL:      r.CoverURL,
					IsExactMatch:  isExactMatch(r.Title, keys),
				})
			}
			mu.Lock()
			candidates = append(candidates, local...)
			mu.Unlock()
		})
		if err != nil {
			logger.Warn("migration candidate search enqueue failed", "plugin", id, "error", err)
			mu.Lock()
			failures = append(failures, id)
			mu.Unlock()
			continue
		}
		futures[id] = fut
	}
	for id, fut := range futures {
		if err := fut.Await(context.Background()); err != nil {
			logger.Warn("migration candidate search future await failed", "plugin", id, "error", err)
			mu.Lock()
			failures = append(failures, id)
			mu.Unlock()
		}
	}
	return candidates, failures, nil
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

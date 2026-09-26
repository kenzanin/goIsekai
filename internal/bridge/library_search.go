package bridge

import (
	"fmt"
	"goisekai/internal/database"
	"sort"
	"strings"
)

// SearchLibrary runs a fuzzy search across the user's library, scoring
// candidates by match quality and returning the top 50. Candidates come from
// the FTS index (title+alt prefix tokens) unioned with a substring scan (mid-
// token partials + descriptions). Ranking is tiered: title/alt-title matches
// (30–100) always outrank description-only matches (1–20).
func (s *AppService) SearchLibrary(q string) ([]SearchHit, error) {
	if strings.TrimSpace(q) == "" {
		return nil, nil
	}
	fts, err := s.db.SearchLibraryFTS(q)
	if err != nil {
		return nil, fmt.Errorf("bridge: fts search: %w", err)
	}
	scan, err := s.db.SearchLibrarySubstring(q)
	if err != nil {
		return nil, fmt.Errorf("bridge: substring search: %w", err)
	}
	seen := make(map[string]bool, len(fts)+len(scan))
	var hits []SearchHit
	lq := strings.ToLower(q)
	process := func(c database.CandidateRow) {
		score := scoreString(c.Title, lq)
		if alts, err := s.db.ListAltTitles(c.MangaRowID); err == nil {
			for _, a := range alts {
				if s := scoreString(a.Title, lq); s > score {
					score = s
				}
			}
		}
		// Description tier: /5 keeps it strictly below the title/alt band
		// (min 30) — exact 20, prefix 16, substring 12, subsequence 6.
		if s := scoreString(c.Description, lq) / 5; s > score {
			score = s
		}
		if ads, err := s.db.ListAltDescriptions(c.MangaRowID); err == nil {
			for _, a := range ads {
				if s := scoreString(a.Description, lq) / 5; s > score {
					score = s
				}
			}
		}
		if score > 0 {
			hits = append(hits, SearchHit{
				PluginID:      c.PluginID,
				SourceMangaID: c.SourceMangaID,
				Title:         c.Title,
				Score:         score,
			})
		}
	}
	for _, c := range fts {
		if !seen[c.MangaRowID] {
			seen[c.MangaRowID] = true
			process(c)
		}
	}
	for _, c := range scan {
		if !seen[c.MangaRowID] {
			seen[c.MangaRowID] = true
			process(c)
		}
	}
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].Score != hits[j].Score {
			return hits[i].Score > hits[j].Score
		}
		return hits[i].Title < hits[j].Title
	})
	if len(hits) > 50 {
		hits = hits[:50]
	}
	return hits, nil
}

// ListAltTitles returns the stored alternative titles for a manga by its
// plugin and source identifiers.
func (s *AppService) ListAltTitles(pluginID, mangaID string) ([]database.AltTitleRow, error) {
	rowID, err := s.db.MangaRowID(pluginID, mangaID)
	if err != nil {
		return nil, fmt.Errorf("bridge: resolve manga: %w", err)
	}
	return s.db.ListAltTitles(rowID)
}

// MangaTitle returns the current main title for a manga.
func (s *AppService) MangaTitle(pluginID, mangaID string) (string, error) {
	return s.db.MangaTitle(pluginID, mangaID)
}

// scoreString returns a relevance score for title matching query lq (already
// lowercased): exact=100, prefix=80, substring=60, subsequence=30, 0 otherwise.
func scoreString(title, lq string) int {
	lt := strings.ToLower(title)
	if lt == lq {
		return 100
	}
	if strings.HasPrefix(lt, lq) {
		return 80
	}
	if strings.Contains(lt, lq) {
		return 60
	}
	if isSubsequence(lq, lt) {
		return 30
	}
	return 0
}

// isSubsequence reports whether the lowercased query chars appear in order
// within s.
func isSubsequence(q, s string) bool {
	if len(q) == 0 {
		return true
	}
	j := 0
	for i := 0; i < len(s) && j < len(q); i++ {
		if s[i] == q[j] {
			j++
		}
	}
	return j == len(q)
}

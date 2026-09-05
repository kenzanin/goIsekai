package bridge

import (
	"fmt"
	"sort"
	"strings"

	"goisekai/internal/database"
	"goisekai/internal/pluginmanager"
)

type SearchHit struct {
	PluginID      string
	SourceMangaID string
	Title         string
	Score         int
}

// AltTitleServers returns every alt-title provider server discovered from
// installed plugins.
func (s *AppService) AltTitleServers() []pluginmanager.AltTitleServerEntry {
	return s.mgr.AltTitleServers()
}

// FetchAltTitles resolves alternative titles for a manga from the given
// provider server, merges them into the database (deduplicating by title),
// rebuilds the FTS index, and returns the full stored list.
func (s *AppService) FetchAltTitles(pluginID, mangaID, server string) ([]database.AltTitleRow, error) {
	rowID, err := s.db.MangaRowID(pluginID, mangaID)
	if err != nil {
		return nil, fmt.Errorf("bridge: resolve manga: %w", err)
	}
	title, err := s.db.MangaTitle(pluginID, mangaID)
	if err != nil {
		return nil, fmt.Errorf("bridge: resolve title: %w", err)
	}
	res, err := s.mgr.GetAltTitles(title, server)
	if err != nil {
		return nil, fmt.Errorf("bridge: fetch alt titles: %w", err)
	}
	if _, err := s.db.AddAltTitles(rowID, res.Titles, res.Source); err != nil {
		return nil, fmt.Errorf("bridge: persist alt titles: %w", err)
	}
	if err := s.db.SyncFTS(rowID); err != nil {
		return nil, fmt.Errorf("bridge: sync fts: %w", err)
	}
	return s.db.ListAltTitles(rowID)
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

// SearchLibrary runs an FTS-backed fuzzy search across the user's library,
// scoring candidates by match quality and returning the top 50.
func (s *AppService) SearchLibrary(q string) ([]SearchHit, error) {
	if strings.TrimSpace(q) == "" {
		return nil, nil
	}
	candidates, err := s.db.SearchLibraryFTS(q)
	if err != nil {
		return nil, fmt.Errorf("bridge: fts search: %w", err)
	}
	if len(candidates) == 0 {
		return nil, nil
	}
	lq := strings.ToLower(q)
	var hits []SearchHit
	for _, c := range candidates {
		score := scoreString(c.Title, lq)
		// Also check alt titles — take the best score.
		alts, err := s.db.ListAltTitles(c.MangaRowID)
		if err == nil {
			for _, a := range alts {
				if s := scoreString(a.Title, lq); s > score {
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

package httpserver

import (
	"strconv"

	"net/http"
	"sync"

	"goisekai/internal/database"
	"goisekai/internal/logger"
	"goisekai/internal/pluginutil"
	"goisekai/pkg/types"
)

// viewMigrate is the search-like picker for migration targets. It looks like
// /view/search but every result has a Migrate button that posts to the
// migration action for the original entry. No search runs on the detail page.
func (s *Server) viewMigrate(w http.ResponseWriter, r *http.Request) {
	pluginID := param(r, "pluginID")
	mangaID := param(r, "mangaID")

	if !s.service.IsInLibrary(pluginID, mangaID) {
		http.Error(w, "not in library", http.StatusBadRequest)
		return
	}

	manga, _, err := s.service.GetMangaDetails(pluginID, mangaID)
	if err != nil {
		manga, _, err = s.service.CachedMangaAndChapters(pluginID, mangaID)
		if err != nil {
			http.Error(w, "failed to load manga", http.StatusBadGateway)
			return
		}
	}

	q := r.URL.Query().Get("q")
	// Prefill with title but don't auto-search: input shows title, results only after Search click.
	displayQ := q
	if displayQ == "" {
		displayQ = manga.Title
	}
	page := 1
	if pStr := r.URL.Query().Get("page"); pStr != "" {
		if p, err := strconv.Atoi(pStr); err == nil && p > 0 {
			page = p
		}
	}
	targetPluginID := r.URL.Query().Get("pluginID")
	plugins, _ := s.service.ListPlugins()
	activeMap := make(map[string]bool)
	for _, p := range plugins {
		activeMap[p.ID] = p.IsActive
	}

	type cand struct {
		PluginID      string
		PluginName    string
		SourceMangaID string
		Title         string
		CoverURL      string
		IsExactMatch  bool
	}
	var cands []cand
	var failures []string
	var searchErr error

	normalizedQ := pluginutil.NormalizeTitle(q)

	if targetPluginID != "" {
		// Single-plugin search.
		if !activeMap[targetPluginID] {
			http.Error(w, "plugin not active", http.StatusBadRequest)
			return
		}
		results, err := s.service.SearchManga(targetPluginID, types.SearchFilter{Query: q, Page: page})
		if err != nil {
			searchErr = err
			failures = append(failures, targetPluginID)
		} else {
			name := pluginName(plugins, targetPluginID)
			for _, r := range results {
				cands = append(cands, cand{PluginID: targetPluginID, PluginName: name, SourceMangaID: r.ID, Title: r.Title, CoverURL: r.CoverURL, IsExactMatch: pluginutil.NormalizeTitle(r.Title) == normalizedQ})
			}
		}
	} else {
		if q == "" {
			// No query yet: show empty, don't hammer servers.
		} else {
			// Search every active plugin with q in parallel.
			var mu sync.Mutex
			var wg sync.WaitGroup
			for _, p := range plugins {
				if !p.IsActive || p.ID == pluginID {
					continue
				}
				wg.Add(1)
				go func(id, name string) {
					defer wg.Done()
					results, err := s.service.SearchManga(id, types.SearchFilter{Query: q, Page: page})
					if err != nil {
						logger.Warn("migrate search failed", "plugin", id, "error", err)
						mu.Lock()
						failures = append(failures, id)
						mu.Unlock()
						return
					}
					if len(results) == 0 {
						return
					}
					var local []cand
					for _, r := range results {
						local = append(local, cand{PluginID: id, PluginName: name, SourceMangaID: r.ID, Title: r.Title, CoverURL: r.CoverURL, IsExactMatch: pluginutil.NormalizeTitle(r.Title) == normalizedQ})
					}
					mu.Lock()
					cands = append(cands, local...)
					mu.Unlock()
				}(p.ID, p.Name)
			}
			wg.Wait()
			if len(cands) == 0 && len(failures) > 0 {
				// Count active candidates to distinguish all-failed vs no results.
				activeCount := 0
				for _, p := range plugins {
					if p.IsActive && p.ID != pluginID {
						activeCount++
					}
				}
				if len(failures) == activeCount {
					searchErr = errAllSourcesFailed
				}
			}
		}
	}

	var candidates []any
	for _, c := range cands {
		candidates = append(candidates, map[string]any{
			"PluginID": c.PluginID, "PluginName": c.PluginName, "SourceMangaID": c.SourceMangaID, "Title": c.Title, "CoverURL": c.CoverURL, "IsExactMatch": c.IsExactMatch,
		})
	}

	data := map[string]any{
		"PluginID":       pluginID,
		"MangaID":        mangaID,
		"Manga":          manga,
		"Plugins":        plugins,
		"Q":              displayQ,
		"TargetPluginID": targetPluginID,
		"Candidates":     candidates,
		"Page":           page,
		"Failures":       failures,
		"SearchError":    "",
	}
	if searchErr != nil {
		data["SearchError"] = searchErr.Error()
	}

	s.renderPage(w, r, "views/migrate", "", data)
}

func pluginName(plugins []database.Plugin, id string) string {
	for _, p := range plugins {
		if p.ID == id {
			if p.Name != "" {
				return p.Name
			}
			return p.ID
		}
	}
	return id
}

var errAllSourcesFailed = errString("sources could not be reached")

type errString string

func (e errString) Error() string { return string(e) }

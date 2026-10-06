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

	manga, sourceChapters, err := s.service.GetMangaDetails(pluginID, mangaID)
	// An empty chapter list is as unhelpful as an error here: this page exists to
	// compare chapter counts, and a dead source returns success with nothing. The
	// DB copy is the truth for an entry we already know is in the library.
	if err != nil || len(sourceChapters) == 0 {
		cachedManga, cachedChapters, cerr := s.service.CachedMangaAndChapters(pluginID, mangaID)
		if cerr == nil && (err != nil || len(cachedChapters) > len(sourceChapters)) {
			manga, sourceChapters = cachedManga, cachedChapters
			err = nil
		}
		if err != nil {
			http.Error(w, "failed to load manga", http.StatusBadGateway)
			return
		}
	}
	sourceChapterCount := len(sourceChapters)

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
		// ChapterCount is -1 when the source could not be asked, so the card can
		// say "unknown" instead of claiming zero chapters.
		ChapterCount int
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
				cands = append(cands, cand{PluginID: targetPluginID, PluginName: name, SourceMangaID: r.ID, Title: r.Title, CoverURL: r.CoverURL, IsExactMatch: pluginutil.NormalizeTitle(r.Title) == normalizedQ, ChapterCount: -1})
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
						local = append(local, cand{PluginID: id, PluginName: name, SourceMangaID: r.ID, Title: r.Title, CoverURL: r.CoverURL, IsExactMatch: pluginutil.NormalizeTitle(r.Title) == normalizedQ, ChapterCount: -1})
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

	// Host-side pagination, 30 per page (same as search).
	const pageSize = 30
	total := len(cands)
	start := min((page-1)*pageSize, total)
	end := min(start+pageSize, total)
	hasNext := end < total
	cands = cands[start:end]

	// A search result carries no chapter count, so the counts have to be asked
	// for - one call per card - to make "will this migration lose chapters?"
	// answerable before the button is pressed rather than after. Only the
	// visible page is asked about, and the plugin cache holds a chapter list for
	// 168h, so revisiting the picker is cheap. A source that cannot be reached
	// keeps -1 and its card says "unknown" rather than lying about having none.
	var wg sync.WaitGroup
	sem := make(chan struct{}, 6)
	for i := range cands {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			chapters, err := s.service.GetChapterList(cands[idx].PluginID, cands[idx].SourceMangaID)
			if err != nil {
				logger.Warn("migrate chapter count failed", "plugin", cands[idx].PluginID, "manga", cands[idx].SourceMangaID, "error", err)
				return
			}
			cands[idx].ChapterCount = len(chapters)
		}(i)
	}
	wg.Wait()

	var candidates []any
	for _, c := range cands {
		candidates = append(candidates, map[string]any{
			"PluginID": c.PluginID, "PluginName": c.PluginName, "SourceMangaID": c.SourceMangaID, "Title": c.Title, "CoverURL": c.CoverURL, "IsExactMatch": c.IsExactMatch,
			"ChapterCount": c.ChapterCount, "SourceChapterCount": sourceChapterCount,
			// Fewer chapters than the entry being moved is the trap this
			// comparison exists to catch: the migration discards the surplus.
			"Fewer": c.ChapterCount >= 0 && sourceChapterCount > 0 && c.ChapterCount < sourceChapterCount,
		})
	}

	data := map[string]any{
		"PluginID":           pluginID,
		"MangaID":            mangaID,
		"Manga":              manga,
		"Plugins":            plugins,
		"Q":                  displayQ,
		"SourceChapterCount": sourceChapterCount,
		"TargetPluginID":     targetPluginID,
		"Candidates":         candidates,
		"Page":               page,
		"HasNext":            hasNext,
		"Failures":           failures,
		"SearchError":        "",
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

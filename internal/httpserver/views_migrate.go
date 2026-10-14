package httpserver

import (
	"context"
	"maps"
	"slices"
	"strconv"
	"strings"

	"net/http"
	"sync"

	"goisekai/internal/database"
	"goisekai/internal/logger"
	"goisekai/internal/pluginutil"
	"goisekai/internal/workers"
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
	// Multi-select: pluginIDs carries a comma-separated list (the form posts one
	// value per checkbox, pagination links join them). Empty = every active
	// plugin. Legacy ?pluginID= single-value links fall through the same split.
	selected := map[string]bool{}
	for _, key := range []string{"pluginIDs", "pluginID"} {
		for _, v := range r.URL.Query()[key] {
			for id := range strings.SplitSeq(v, ",") {
				if id = strings.TrimSpace(id); id != "" {
					selected[id] = true
				}
			}
		}
		if len(selected) > 0 {
			break
		}
	}
	plugins, _ := s.service.ListPlugins()
	activeMap := make(map[string]bool)
	for _, p := range plugins {
		activeMap[p.ID] = p.IsActive
	}
	for id := range selected {
		if !activeMap[id] {
			http.Error(w, "plugin not active", http.StatusBadRequest)
			return
		}
	}
	// Search targets: the checked plugins, or all active ones when none are
	// checked. The entry's own source can never be a migration target.
	targets := make([]string, 0, len(plugins))
	for _, p := range plugins {
		if !p.IsActive || p.ID == pluginID || (len(selected) > 0 && !selected[p.ID]) {
			continue
		}
		targets = append(targets, p.ID)
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

	if q != "" {
		// Search every target in parallel via worker pool; no query yet shows empty rather than
		// hammering servers.
		var mu sync.Mutex
		futures := make(map[string]*workers.Future)
		for _, id := range targets {
			fut, err := s.service.EnqueueSearchManga(context.Background(), id, types.SearchFilter{Query: q, Page: 1}, func(ctx context.Context, results []types.Manga, err error) {
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
				name := pluginName(plugins, id)
				var local []cand
				for _, r := range results {
					local = append(local, cand{PluginID: id, PluginName: name, SourceMangaID: r.ID, Title: r.Title, CoverURL: r.CoverURL, IsExactMatch: pluginutil.NormalizeTitle(r.Title) == normalizedQ, ChapterCount: -1})
				}
				mu.Lock()
				cands = append(cands, local...)
				mu.Unlock()
			})
			if err != nil {
				logger.Warn("migrate search enqueue failed", "plugin", id, "error", err)
				mu.Lock()
				failures = append(failures, id)
				mu.Unlock()
				continue
			}
			futures[id] = fut
		}
		// Wait for all searches to complete
		for id, fut := range futures {
			if err := fut.Await(context.Background()); err != nil {
				logger.Warn("migrate search future await failed", "plugin", id, "error", err)
				mu.Lock()
				failures = append(failures, id)
				mu.Unlock()
			}
		}
		if len(cands) == 0 && len(failures) > 0 && len(failures) == len(targets) {
			// Every target failed: distinct from a genuine no-results.
			searchErr = errAllSourcesFailed
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
	futures := make(map[int]*workers.Future)
	for i := range cands {
		fut, err := s.service.EnqueueGetChapterList(context.Background(), cands[i].PluginID, cands[i].SourceMangaID, func(ctx context.Context, chapters []types.Chapter, err error) {
			if err != nil {
				logger.Warn("migrate chapter count failed", "plugin", cands[i].PluginID, "manga", cands[i].SourceMangaID, "error", err)
				return
			}
			cands[i].ChapterCount = len(chapters)
		})
		if err != nil {
			logger.Warn("migrate chapter enqueue failed", "plugin", cands[i].PluginID, "manga", cands[i].SourceMangaID, "error", err)
			continue
		}
		futures[i] = fut
	}
	// Wait for all chapter fetches to complete
	for i, fut := range futures {
		if err := fut.Await(context.Background()); err != nil {
			logger.Warn("migrate chapter future await failed", "plugin", cands[i].PluginID, "manga", cands[i].SourceMangaID, "error", err)
		}
	}

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
		"PluginIDs":          strings.Join(slices.Sorted(maps.Keys(selected)), ","),
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

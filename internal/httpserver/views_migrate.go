package httpserver

import (
	"maps"
	"slices"
	"strconv"
	"strings"

	"net/http"
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

	candidates, failures, searchErr, hasNext, fatal := s.collectMigrationPage(
		pluginID, mangaID, q, targets, page, sourceChapterCount)
	if fatal != nil {
		http.Error(w, fatal.Error(), http.StatusBadGateway)
		return
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

var errAllSourcesFailed = errString("sources could not be reached")

type errString string

func (e errString) Error() string { return string(e) }

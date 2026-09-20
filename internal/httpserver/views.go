package httpserver

import (
	"errors"
	"github.com/goccy/go-json"
	"net/http"
	"strconv"

	"goisekai/internal/bridge"
	"goisekai/internal/database"
	"goisekai/internal/hostnet"
	"goisekai/pkg/types"
	"strings"
)

// viewHistory renders the reading history page.
func (s *Server) viewHistory(w http.ResponseWriter, r *http.Request) {
	history, err := s.service.GetReadHistory()
	if err != nil {
		s.logger.Error("history list", "error", err)
	}
	var entries []database.HistoryEntry
	names, icons := s.pluginDisplayMaps()
	for _, h := range history {
		h.PluginName = h.PluginID
		if name := names[h.PluginID]; name != "" {
			h.PluginName = name
		}
		h.PluginIcon = icons[h.PluginID]
		entries = append(entries, h)
	}

	const pageSize = 24
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	total := len(entries)
	start := min((page-1)*pageSize, total)
	end := min(start+pageSize, total)
	s.renderPage(w, r, "views/history", "history", map[string]any{
		"History":    entries[start:end],
		"Page":       page,
		"TotalPages": max((total+pageSize-1)/pageSize, 1),
	})
}

// searchPageSize is the number of results per search page. 30 fills the
// 6-column results grid with 5 rows.
const searchPageSize = 30

// viewSearch renders the search form and, when q+pluginID are present, results.
func (s *Server) viewSearch(w http.ResponseWriter, r *http.Request) {
	plugins, err := s.service.ListPlugins()
	if err != nil {
		s.logger.Error("plugin list", "error", err)
	}
	q := r.URL.Query().Get("q")
	pluginID := r.URL.Query().Get("pluginID")
	if pluginID == "" {
		// Default to the first active plugin so the genre picker renders on a
		// fresh page load, not only once a plugin has been submitted.
		for _, p := range plugins {
			if p.IsActive {
				pluginID = p.ID
				break
			}
		}
	}
	genre := r.URL.Query().Get("genre")
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	var genres []bridge.Genre
	if pluginID != "" {
		genres, _ = s.service.ListGenres(pluginID)
	}
	var results []types.Manga
	challenge := false
	if (q != "" || genre != "") && pluginID != "" {
		results, err = s.service.SearchManga(pluginID, types.SearchFilter{Query: q, Page: page, Genres: []string{genre}})
		if err != nil {
			if _, ok := errors.AsType[*hostnet.ChallengeError](err); ok {
				challenge = true
				results = nil
				s.logger.Warn("search blocked by challenge", "plugin", pluginID, "q", q)
			} else {
				s.logger.Error("search", "error", err, "plugin", pluginID, "q", q)
			}
		}
	}
	pageSize := searchPageSize
	// Host-side pagination: plugins return ALL matching results; slice out the
	// requested page here so the template never renders more than one page.
	total := len(results)
	start := min((page-1)*pageSize, total)
	end := min(start+pageSize, total)
	pluginName := pluginID
	pluginIcon := ""
	if pluginID != "" {
		names, icons := s.pluginDisplayMaps()
		if name := names[pluginID]; name != "" {
			pluginName = name
		}
		pluginIcon = icons[pluginID]
	}
	s.renderPage(w, r, "views/search", "search", map[string]any{
		"Plugins":    plugins,
		"Q":          q,
		"PluginID":   pluginID,
		"Genres":     genres,
		"Genre":      genre,
		"PluginName": pluginName,
		"PluginIcon": pluginIcon,
		"Results":    results[start:end],
		"Page":       page,
		"TotalPages": max((total+pageSize-1)/pageSize, 1),
		"HasNext":    end < total,
		"ThumbRatio": s.service.PluginMeta(pluginID).ThumbRatio,
		"Challenge":  challenge,
	})
}

// viewAbout renders the project README as the About page. The markdown is
// converted server-side on every request; the file is tiny and local.
func (s *Server) viewAbout(w http.ResponseWriter, r *http.Request) {
	s.renderPage(w, r, "views/about", "about", map[string]any{
		"Content": renderMarkdown(loadReadme()),
	})
}

// viewLogs renders the in-memory log buffer with a 2s HTMX poll.
func (s *Server) viewLogs(w http.ResponseWriter, r *http.Request) {
	limit := 500
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = min(n, 2000)
		}
	}
	logs := s.service.GetLogs()
	filter := r.URL.Query().Get("filter") // "", "app" or "plugins"
	switch filter {
	case "app":
		out := logs[:0]
		for _, l := range logs {
			if !strings.Contains(l, " plugin=") {
				out = append(out, l)
			}
		}
		logs = out
	case "plugins":
		out := logs[:0]
		for _, l := range logs {
			if strings.Contains(l, " plugin=") {
				out = append(out, l)
			}
		}
		logs = out
	}
	if len(logs) > limit {
		logs = logs[len(logs)-limit:]
	}
	s.renderPage(w, r, "views/logs", "logs", map[string]any{"Logs": logs, "Limit": limit, "Limits": []int{100, 250, 500, 1000, 2000}, "Filter": filter})
}

// jsonMarshalLogs writes lines as a JSON array.
func jsonMarshalLogs(w http.ResponseWriter, lines []string) error {
	enc := json.NewEncoder(w)
	return enc.Encode(lines)
}

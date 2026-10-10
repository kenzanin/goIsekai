package httpserver

import (
	"github.com/goccy/go-json"
	"net/http"
	"strconv"

	"goisekai/internal/database"
	"goisekai/internal/version"
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

// viewAbout renders the project README as the About page. The markdown is
// converted server-side on every request; the file is tiny and local.
func (s *Server) viewAbout(w http.ResponseWriter, r *http.Request) {
	s.renderPage(w, r, "views/about", "about", map[string]any{
		"Content": renderMarkdown(loadReadme()),
		"Version": version.String(),
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
	filter := r.URL.Query().Get("filter") // "", "app", "plugins", "warn" or "error"
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
	case "warn", "error":
		needle := " " + strings.ToUpper(filter) + " "
		out := logs[:0]
		for _, l := range logs {
			if strings.Contains(l, needle) {
				out = append(out, l)
			}
		}
		logs = out
	}
	if len(logs) > limit {
		logs = logs[len(logs)-limit:]
	}
	// Newest first: reverse in place.
	for i, j := 0, len(logs)-1; i < j; i, j = i+1, j-1 {
		logs[i], logs[j] = logs[j], logs[i]
	}
	s.renderPage(w, r, "views/logs", "logs", map[string]any{"Logs": logs, "Limit": limit, "Limits": []int{100, 250, 500, 1000, 2000}, "Filter": filter})
}

// jsonMarshalLogs writes lines as a JSON array.
func jsonMarshalLogs(w http.ResponseWriter, lines []string) error {
	enc := json.NewEncoder(w)
	return enc.Encode(lines)
}

package httpserver

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

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
	metas := s.service.PluginMetas()
	for _, h := range history {
		h.PluginName = h.PluginID
		if m, ok := metas[h.PluginID]; ok {
			if m.Name != "" {
				h.PluginName = m.Name
			}
			if m.Logo != "" {
				h.PluginIcon = resolveLogoURL(m.Logo, h.PluginID)
			}
		}
		entries = append(entries, h)
	}
	s.renderPage(w, r, "views/history", "history", map[string]any{"History": entries})
}

// viewSearch renders the search form and, when q+pluginID are present, results.
func (s *Server) viewSearch(w http.ResponseWriter, r *http.Request) {
	plugins, err := s.service.ListPlugins()
	if err != nil {
		s.logger.Error("plugin list", "error", err)
	}
	q := r.URL.Query().Get("q")
	pluginID := r.URL.Query().Get("pluginID")
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	var results []types.Manga
	challenge := false
	if q != "" && pluginID != "" {
		results, err = s.service.SearchManga(pluginID, types.SearchFilter{Query: q, Page: page})
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
	pageSize := s.service.PluginMeta(pluginID).SearchPageSize
	if pageSize <= 0 {
		pageSize = 24
	}
	// Host-side pagination: plugins return ALL matching results (the search
	// contract per plugin metadata search_page_size); slice out the requested
	// page here so the template never renders more than one page.
	total := len(results)
	start := min((page-1)*pageSize, total)
	end := min(start+pageSize, total)
	pluginName := pluginID
	pluginIcon := ""
	if pluginID != "" {
		if m, ok := s.service.PluginMetas()[pluginID]; ok {
			if m.Name != "" {
				pluginName = m.Name
			}
			if m.Logo != "" {
				pluginIcon = resolveLogoURL(m.Logo, pluginID)
			}
		}
	}
	s.renderPage(w, r, "views/search", "search", map[string]any{
		"Plugins":    plugins,
		"Q":          q,
		"PluginID":   pluginID,
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

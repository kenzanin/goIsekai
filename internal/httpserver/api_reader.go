package httpserver

import (
	"github.com/goccy/go-json"
	"net/http"
)

// apiHistoryEntry is the JSON shape for GET /history.
type apiHistoryEntry struct {
	PluginID      string `json:"plugin_id"`
	SourceMangaID string `json:"source_manga_id"`
	Title         string `json:"title"`
	CoverURL      string `json:"cover_url"`
	TotalChapters int    `json:"total_chapters"`
	ReadChapters  int    `json:"read_chapters"`
	LastReadAt    string `json:"last_read_at"`
}

// apiHistory mirrors viewHistory: recently-read manga.
func (s *Server) apiHistory(w http.ResponseWriter, r *http.Request) {
	history, err := s.service.GetReadHistory()
	if err != nil {
		s.logger.Error("api history", "error", err)
		writeErr(w, http.StatusInternalServerError, "failed to load history")
		return
	}
	entries := make([]apiHistoryEntry, 0, len(history))
	for _, h := range history {
		entries = append(entries, apiHistoryEntry{
			PluginID:      h.PluginID,
			SourceMangaID: h.SourceMangaID,
			Title:         h.Title,
			CoverURL:      h.CoverURL,
			TotalChapters: h.TotalChapters,
			ReadChapters:  h.ReadChapters,
			LastReadAt:    h.LastReadAt.Format("2006-01-02T15:04:05Z"),
		})
	}
	writeJSON(w, http.StatusOK, entries)
}

// apiPluginsItem is the JSON shape for a single plugin in GET /plugins.
type apiPluginsItem struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Version    string  `json:"version"`
	IsActive   bool    `json:"is_active"`
	ThumbRatio float64 `json:"thumb_ratio"`
}

// apiPlugins lists all registered plugins.
func (s *Server) apiPlugins(w http.ResponseWriter, r *http.Request) {
	plugins, err := s.service.ListPlugins()
	if err != nil {
		s.logger.Error("api plugins list", "error", err)
		writeErr(w, http.StatusInternalServerError, "failed to load plugins")
		return
	}
	metas := s.service.PluginMetas()
	items := make([]apiPluginsItem, 0, len(plugins))
	for _, p := range plugins {
		item := apiPluginsItem{
			ID:       p.ID,
			Name:     p.Name,
			Version:  p.Version,
			IsActive: p.IsActive,
		}
		if m, ok := metas[p.ID]; ok {
			item.ThumbRatio = m.ThumbRatio
			if m.Name != "" {
				item.Name = m.Name
			}
		}
		items = append(items, item)
	}
	writeJSON(w, http.StatusOK, items)
}

// apiToggleLibrary mirrors handleToggleLibrary: flips in-library and reports state.
func (s *Server) apiToggleLibrary(w http.ResponseWriter, r *http.Request) {
	pluginID := param(r, "pluginID")
	mangaID := param(r, "mangaID")
	if err := s.service.ToggleLibraryItem(pluginID, mangaID); err != nil {
		s.logger.Error("api toggle library", "pluginID", pluginID, "mangaID", mangaID, "error", err)
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{
		"in_library": s.service.IsInLibrary(pluginID, mangaID),
	})
}

// apiMarkChapterReadRequest is the optional POST body for /chapters/read.
type apiMarkChapterReadRequest struct {
	Read *bool `json:"read"`
}

// apiMarkChapterRead mirrors handleMarkChapterRead with optional read:bool body.
func (s *Server) apiMarkChapterRead(w http.ResponseWriter, r *http.Request) {
	pluginID := param(r, "pluginID")
	mangaID := param(r, "mangaID")
	chapterID := param(r, "chapterID")
	readFlag := true
	if r.ContentLength > 0 {
		var req apiMarkChapterReadRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if req.Read != nil {
			readFlag = *req.Read
		}
	}
	if readFlag {
		if err := s.service.MarkChapterRead(pluginID, mangaID, chapterID); err != nil {
			s.logger.Error("api mark read", "error", err)
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
	} else {
		if err := s.service.ResetChapterProgress(pluginID, mangaID, chapterID); err != nil {
			s.logger.Error("api reset read", "error", err)
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]bool{"is_read": readFlag})
}

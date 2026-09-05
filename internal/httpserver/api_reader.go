package httpserver

import (
	"encoding/json"
	"net/http"
	"net/url"
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

// apiSetProgressRequest is the POST body for /progress.
type apiSetProgressRequest struct {
	Page int `json:"page"`
}

// apiSetProgress mirrors handleSetChapterProgress: records last-read page.
func (s *Server) apiSetProgress(w http.ResponseWriter, r *http.Request) {
	pluginID := param(r, "pluginID")
	mangaID := param(r, "mangaID")
	chapterID := param(r, "chapterID")
	var req apiSetProgressRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Page < 0 {
		writeErr(w, http.StatusBadRequest, "page must be >= 0")
		return
	}
	if err := s.service.SetChapterProgress(pluginID, mangaID, chapterID, req.Page); err != nil {
		s.logger.Error("api set progress", "error", err)
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"page": req.Page, "total_pages": 0})
}

// apiImage proxies an image through the bridge and returns the raw bytes with
// the detected Content-Type. Mirrors handleImage but uses path params and the
// incoming Referer header instead of query params.
func (s *Server) apiImage(w http.ResponseWriter, r *http.Request) {
	pluginID := param(r, "pluginID")
	mangaID := param(r, "mangaID")
	chapterID := param(r, "chapterID")
	q := r.URL.Query()
	urlStr := q.Get("url")
	if urlStr == "" {
		writeErr(w, http.StatusBadRequest, "missing required query parameter: url")
		return
	}
	if unesc, err := url.QueryUnescape(urlStr); err == nil {
		urlStr = unesc
	}

	var headers map[string]string
	if ref := r.Header.Get("Referer"); ref != "" {
		headers = map[string]string{"Referer": ref}
	}

	s.logger.Debug("api image", "pluginID", pluginID, "url", urlStr, "mangaID", mangaID, "chapterID", chapterID)
	data, err := s.service.GetImage(pluginID, urlStr, headers, mangaID, chapterID)
	if err != nil {
		s.logger.Error("api image fetch", "url", urlStr, "pluginID", pluginID, "error", err)
		writeErr(w, http.StatusBadGateway, "image fetch failed")
		return
	}
	w.Header().Set("Content-Type", http.DetectContentType(data))
	w.Header().Set("Cache-Control", "public, max-age=604800")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

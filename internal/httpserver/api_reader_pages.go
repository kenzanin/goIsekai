package httpserver

import (
	"context"
	"net/http"
	"net/url"

	"github.com/goccy/go-json"

	"goisekai/internal/bridge"
)

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
	if _, err := s.service.EnqueueSetChapterProgress(context.Background(), pluginID, mangaID, chapterID, req.Page); err != nil {
		s.logger.Error("api set progress enqueue", "error", err)
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"page": req.Page})
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
	data, err := s.service.GetImage(r.Context(), pluginID, urlStr, headers, mangaID, chapterID, bridge.PrioLow)
	if err != nil {
		if r.Context().Err() != nil {
			// Abandoned, not failed: the client is gone, so this is not an
			// upstream fault and must not be logged as one.
			s.logger.Debug("api image fetch abandoned", "url", urlStr, "pluginID", pluginID, "reason", err)
			return
		}
		s.logger.Error("api image fetch", "url", urlStr, "pluginID", pluginID, "error", err)
		writeErr(w, http.StatusBadGateway, "image fetch failed")
		return
	}
	w.Header().Set("Content-Type", bridge.ImageContentType(data))
	w.Header().Set("Cache-Control", "public, max-age=604800")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

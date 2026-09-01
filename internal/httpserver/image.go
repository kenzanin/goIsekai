package httpserver

import (
	"net/http"
)

// registerImageRoutes mounts the binary image proxy endpoint. Pages reference
// images as <img src="/image?pluginID=...&url=...&mangaID=...&chapterID=...">;
// the bridge owns the per-plugin/manga/chapter disk cache, so this handler is
// just pass-through plus MIME sniffing and long-lived caching headers.
func (s *Server) registerImageRoutes() {
	s.Router.Get("/image", s.handleImage)
}

// handleImage proxies one image fetch through the bridge and serves the bytes
// with a sniffed Content-Type and a week-long cache lifetime.
func (s *Server) handleImage(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	pluginID := q.Get("pluginID")
	url := q.Get("url")
	if pluginID == "" || url == "" {
		http.Error(w, "pluginID and url query params are required", http.StatusBadRequest)
		return
	}
	mangaID := q.Get("mangaID")
	chapterID := q.Get("chapterID")

	// Per-page headers flow through the reader as query params. Currently only
	// Referer matters (some CDNs 403 without it).
	var headers map[string]string
	if ref := q.Get("referer"); ref != "" {
		headers = map[string]string{"Referer": ref}
	}

	s.logger.Debug("image request", "pluginID", pluginID, "url", url, "mangaID", mangaID, "chapterID", chapterID)
	data, err := s.service.GetImage(pluginID, url, headers, mangaID, chapterID)
	if err != nil {
		s.logger.Error("image fetch", "url", url, "pluginID", pluginID, "error", err)
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	// http.DetectContentType only inspects the first 512 bytes; pass the whole
	// slice and let it look at the prefix.
	w.Header().Set("Content-Type", http.DetectContentType(data))
	w.Header().Set("Cache-Control", "public, max-age=604800")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

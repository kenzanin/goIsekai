package httpserver

import (
	"io"
	"net/http"
	"strings"

	"github.com/goccy/go-json"
)

// renderCoverModal fetches cached cover + alt-covers from the DB (no plugin
// detail fetch) and renders the cover-picker partial. It accepts an optional
// toast message that the JS submit handler reads from the X-Toast header to
// surface a success confirmation to the user.
func (s *Server) renderCoverModal(w http.ResponseWriter, pluginID, mangaID, toast string) error {
	cached, err := s.service.CachedCover(pluginID, mangaID)
	if err != nil {
		return err
	}
	altCovers, err := s.service.ListAltCovers(pluginID, mangaID)
	if err != nil {
		return err
	}
	coverVersions := make(map[string]string, len(altCovers)+1)
	for _, c := range altCovers {
		if c.URL != "" {
			if v := s.service.CoverCacheVersion(pluginID, c.URL); v != "" {
				coverVersions[c.URL] = v
			}
		}
	}
	if cached.CoverURL != "" {
		if v := s.service.CoverCacheVersion(pluginID, cached.CoverURL); v != "" {
			coverVersions[cached.CoverURL] = v
		}
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if toast != "" {
		w.Header().Set("X-Toast", toast)
	}
	return s.engine.RenderPartial(w, "partials/cover_modal", map[string]any{
		"PluginID":      pluginID,
		"MangaID":       mangaID,
		"Manga":         cached,
		"AltCovers":     altCovers,
		"CoverVersions": coverVersions,
		"CurrentTitle":  cached.Title,
		csrfField:       s.csrfToken,
	})
}

// handleSaveReaderPrefs stores the per-manga reader prefs JSON blob.
// Body is the raw JSON object; empty body clears back to global defaults.
// Answers 204 — the reader fires it debounced in the background.
func (s *Server) handleSaveReaderPrefs(w http.ResponseWriter, r *http.Request) {
	pluginID := param(r, "pluginID")
	mangaID := param(r, "mangaID")
	raw, err := io.ReadAll(io.LimitReader(r.Body, 4096))
	if err != nil {
		http.Error(w, "read body", http.StatusBadRequest)
		return
	}
	prefs := strings.TrimSpace(string(raw))
	if prefs == "" {
		prefs = ""
	} else {
		var v map[string]any
		if err := json.Unmarshal([]byte(prefs), &v); err != nil {
			http.Error(w, "prefs must be a JSON object", http.StatusBadRequest)
			return
		}
		raw, err := json.Marshal(v)
		if err != nil {
			http.Error(w, "encode prefs", http.StatusBadRequest)
			return
		}
		prefs = string(raw)
	}
	if err := s.service.SaveReaderPrefs(pluginID, mangaID, prefs); err != nil {
		s.logger.Error("save reader prefs", "pluginID", pluginID, "mangaID", mangaID, "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

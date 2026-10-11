package httpserver

import (
	"io"
	"net/http"
	"strings"

	"github.com/goccy/go-json"
)

// handleFetchCovers gathers alternative cover candidates from enrichment
// sources and stores them for the picker.
func (s *Server) handleFetchCovers(w http.ResponseWriter, r *http.Request) {
	pluginID := param(r, "pluginID")
	mangaID := param(r, "mangaID")
	if err := r.ParseForm(); err != nil {
		s.logger.Error("fetch covers: parse form", "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	title := r.FormValue("manga_title")
	if err := s.service.FetchCovers(pluginID, mangaID, title); err != nil {
		s.logger.Error("fetch covers", "pluginID", pluginID, "mangaID", mangaID, "error", err)
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	s.viewMangaDetail(w, r)
}

// handleSetCover promotes one alternative cover URL to the manga's cover.
func (s *Server) handleSetCover(w http.ResponseWriter, r *http.Request) {
	pluginID := param(r, "pluginID")
	mangaID := param(r, "mangaID")
	if err := r.ParseForm(); err != nil {
		s.logger.Error("set cover: parse form", "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	url := r.FormValue("url")
	if err := s.service.SetCover(pluginID, mangaID, url); err != nil {
		s.logger.Error("set cover", "pluginID", pluginID, "mangaID", mangaID, "url", url, "error", err)
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	s.toastRedirect(w, r, "/view/manga/"+pluginID+"/"+mangaID, "Cover updated")
}

// handleRefetchCover re-downloads a manga's cover, busting the caches.
func (s *Server) handleRefetchCover(w http.ResponseWriter, r *http.Request) {
	pluginID := param(r, "pluginID")
	mangaID := param(r, "mangaID")
	if err := s.service.RefetchCover(pluginID, mangaID); err != nil {
		s.logger.Error("refetch cover", "pluginID", pluginID, "mangaID", mangaID, "error", err)
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	s.toastRedirect(w, r, "/view/manga/"+pluginID+"/"+mangaID, "Cover re-fetched")
}

// handleToggleCoverDim toggles the cover dim overlay on a manga.
func (s *Server) handleToggleCoverDim(w http.ResponseWriter, r *http.Request) {
	pluginID := param(r, "pluginID")
	mangaID := param(r, "mangaID")
	if err := s.service.ToggleCoverDim(pluginID, mangaID); err != nil {
		s.logger.Error("toggle cover dim", "pluginID", pluginID, "mangaID", mangaID, "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.hxRedirect(w, r, "/view/manga/"+pluginID+"/"+mangaID)
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

// handleClearCovers drops the stored alternative cover candidates so the
// next fetch re-pulls them fresh from enrichment sources.
func (s *Server) handleClearCovers(w http.ResponseWriter, r *http.Request) {
	pluginID := param(r, "pluginID")
	mangaID := param(r, "mangaID")
	if err := s.service.ClearCovers(pluginID, mangaID); err != nil {
		s.logger.Error("clear covers", "pluginID", pluginID, "mangaID", mangaID, "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.toastRedirect(w, r, "/view/manga/"+pluginID+"/"+mangaID, "Cover candidates cleared")
}

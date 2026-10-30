package httpserver

import (
	"net/http"

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
	s.logger.Debug("fetch-covers request", "pluginID", pluginID, "mangaID", mangaID, "title", title)
	if err := s.service.FetchCovers(pluginID, mangaID, title); err != nil {
		s.logger.Error("fetch covers", "pluginID", pluginID, "mangaID", mangaID, "error", err)
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	// Separation of concerns: the Covers button only refreshes the picker
	// modal, not the whole detail page. Read the cached manga + alt covers —
	// no plugin detail fetch — and render just the modal partial.
	cached, err := s.service.CachedCover(pluginID, mangaID)
	if err != nil {
		s.logger.Error("fetch covers: load cached manga", "pluginID", pluginID, "mangaID", mangaID, "error", err)
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	altCovers, err := s.service.ListAltCovers(pluginID, mangaID)
	if err != nil {
		s.logger.Error("fetch covers: list alt covers", "pluginID", pluginID, "mangaID", mangaID, "error", err)
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
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
	if err := s.engine.RenderPartial(w, "partials/cover_modal", map[string]any{
		"PluginID":      pluginID,
		"MangaID":       mangaID,
		"Manga":         cached,
		"AltCovers":     altCovers,
		"CoverVersions": coverVersions,
		"CurrentTitle":  cached.Title,
		csrfField:       s.csrfToken,
	}); err != nil {
		s.logger.Error("render cover modal", "error", err)
		http.Error(w, "render failed", http.StatusInternalServerError)
	}
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
	s.logger.Debug("set-cover request", "pluginID", pluginID, "mangaID", mangaID, "url", url)
	if err := s.service.SetCover(pluginID, mangaID, url); err != nil {
		s.logger.Error("set cover", "pluginID", pluginID, "mangaID", mangaID, "url", url, "error", err)
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	// Swap modal in-place so the detail page (and its GetMangaDetail call)
	// stays untouched — same pattern as fetch-covers. The caller already
	// swapped #cover-modal in when it opened the picker, so swapping it
	// out again closes the picker without a reload.
	if err := s.renderCoverModal(w, pluginID, mangaID, "Cover updated"); err != nil {
		s.logger.Error("render cover modal after set", "error", err)
		http.Error(w, "render failed", http.StatusInternalServerError)
	}
}

// handleRefetchCover re-downloads a manga's cover, busting the caches.
func (s *Server) handleRefetchCover(w http.ResponseWriter, r *http.Request) {
	pluginID := param(r, "pluginID")
	mangaID := param(r, "mangaID")
	s.logger.Debug("refetch-cover request", "pluginID", pluginID, "mangaID", mangaID)
	if err := s.service.RefetchCover(pluginID, mangaID); err != nil {
		s.logger.Error("refetch cover", "pluginID", pluginID, "mangaID", mangaID, "error", err)
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	// In-place modal swap: avoids re-invoking the plugin's GetMangaDetail.
	if err := s.renderCoverModal(w, pluginID, mangaID, "Cover re-fetched"); err != nil {
		s.logger.Error("render cover modal after refetch", "error", err)
		http.Error(w, "render failed", http.StatusInternalServerError)
	}
}

// handleToggleCoverDim toggles the cover dim overlay flag and returns the
// current dim state as JSON — the client updates the overlay in place.
// No page redirect: a redirect re-renders the whole detail page, which
// re-invokes the plugin's GetMangaDetail and repopulates alt details.
func (s *Server) handleToggleCoverDim(w http.ResponseWriter, r *http.Request) {
	pluginID := param(r, "pluginID")
	mangaID := param(r, "mangaID")
	s.logger.Debug("toggle-cover-dim request", "pluginID", pluginID, "mangaID", mangaID)
	dim, err := s.service.ToggleCoverDim(pluginID, mangaID)
	if err != nil {
		s.logger.Error("toggle cover dim", "pluginID", pluginID, "mangaID", mangaID, "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"dim": dim})
}

// handleClearCovers drops the stored alternative cover candidates so the
// next fetch re-pulls them fresh from enrichment sources.
func (s *Server) handleClearCovers(w http.ResponseWriter, r *http.Request) {
	pluginID := param(r, "pluginID")
	mangaID := param(r, "mangaID")
	s.logger.Debug("clear-covers request", "pluginID", pluginID, "mangaID", mangaID)
	if err := s.service.ClearCovers(pluginID, mangaID); err != nil {
		s.logger.Error("clear covers", "pluginID", pluginID, "mangaID", mangaID, "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	// In-place modal swap: avoids re-invoking the plugin's GetMangaDetail.
	if err := s.renderCoverModal(w, pluginID, mangaID, "Cover candidates cleared"); err != nil {
		s.logger.Error("render cover modal after clear", "error", err)
		http.Error(w, "render failed", http.StatusInternalServerError)
	}
}

package httpserver

import (
	"net/http"
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

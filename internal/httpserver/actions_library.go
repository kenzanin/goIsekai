package httpserver

import (
	"net/http"
)

// handleToggleLibrary flips a manga's in-library flag.
func (s *Server) handleToggleLibrary(w http.ResponseWriter, r *http.Request) {
	pluginID := param(r, "pluginID")
	mangaID := param(r, "mangaID")
	if err := s.service.ToggleLibraryItem(pluginID, mangaID); err != nil {
		s.logger.Error("toggle library", "pluginID", pluginID, "mangaID", mangaID, "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.hxRedirect(w, "/view/manga/"+pluginID+"/"+mangaID)
}

// handleSyncManga re-fetches one library manga now. A refresh inside the
// auto-update threshold is refused; the button already says why.
func (s *Server) handleSyncManga(w http.ResponseWriter, r *http.Request) {
	pluginID := param(r, "pluginID")
	mangaID := param(r, "mangaID")
	if err := s.service.SyncManga(pluginID, mangaID, false); err != nil {
		s.logger.Warn("sync manga", "plugin", pluginID, "manga", mangaID, "error", err)
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	s.hxRedirect(w, "/view/manga/"+pluginID+"/"+mangaID)
}

// handleSync re-fetches chapter lists for every library manga.
func (s *Server) handleSync(w http.ResponseWriter, r *http.Request) {
	if err := s.service.SyncLibrary(); err != nil {
		s.logger.Error("sync library", "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.hxRedirect(w, "/view/library")
}

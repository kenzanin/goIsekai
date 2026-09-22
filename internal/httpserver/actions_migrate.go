package httpserver

import (
	"net/http"
)

// handleMigrateSource repoints a library entry at another source.
func (s *Server) handleMigrateSource(w http.ResponseWriter, r *http.Request) {
	pluginID := param(r, "pluginID")
	mangaID := param(r, "mangaID")
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	targetPluginID := r.FormValue("targetPluginID")
	targetMangaID := r.FormValue("targetMangaID")
	if targetPluginID == "" || targetMangaID == "" {
		http.Error(w, "missing target", http.StatusBadRequest)
		return
	}
	if err := s.service.MigrateMangaSource(pluginID, mangaID, targetPluginID, targetMangaID); err != nil {
		s.logger.Error("migrate source", "from", pluginID+"/"+mangaID, "to", targetPluginID+"/"+targetMangaID, "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	// Redirect to the new location.
	s.hxRedirect(w, "/view/manga/"+targetPluginID+"/"+targetMangaID)
}

package httpserver

import (
	"encoding/json"
	"net/http"
)

// jobRef is the immediate response for an enqueued long-running action.
// Shape is fixed by design.md: {"status":"ok","job_id":"<id>"} — field order
// matters, so this is a struct rather than a map (maps marshal alphabetically).
type jobRef struct {
	Status string `json:"status"`
	JobID  string `json:"job_id"`
	// URL is set only by actions whose result the browser can fetch. Omitted
	// elsewhere so every other job response keeps the shape design.md fixes.
	URL string `json:"url,omitempty"`
}

func writeJobRef(w http.ResponseWriter, jobID string) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(jobRef{Status: "ok", JobID: jobID})
}

// handleToggleLibrary flips a manga's in-library flag.
func (s *Server) handleToggleLibrary(w http.ResponseWriter, r *http.Request) {
	pluginID := param(r, "pluginID")
	mangaID := param(r, "mangaID")
	if err := s.service.ToggleLibraryItem(pluginID, mangaID); err != nil {
		s.logger.Error("toggle library", "pluginID", pluginID, "mangaID", mangaID, "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.hxRedirect(w, r, "/view/manga/"+pluginID+"/"+mangaID)
}

// handleSyncManga re-fetches one library manga on demand. Always allowed; the
// sync stamps updated_at, so the auto-updater then leaves it alone until it
// goes stale again.
func (s *Server) handleSyncManga(w http.ResponseWriter, r *http.Request) {
	pluginID := param(r, "pluginID")
	mangaID := param(r, "mangaID")
	jobID, err := s.service.EnqueueSyncManga(r.Context(), pluginID, mangaID)
	if err != nil {
		s.logger.Warn("sync manga", "plugin", pluginID, "manga", mangaID, "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJobRef(w, jobID)
}

// handleSync re-fetches chapter lists for every library manga.
func (s *Server) handleSync(w http.ResponseWriter, r *http.Request) {
	jobID, err := s.service.EnqueueSyncLibrary(r.Context())
	if err != nil {
		s.logger.Error("sync library", "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJobRef(w, jobID)
}

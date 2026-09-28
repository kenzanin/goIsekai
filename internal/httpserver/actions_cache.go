package httpserver

import (
	"encoding/json"
	"net/http"
)

// handleClearLogs empties the in-memory log buffer.
func (s *Server) handleClearLogs(w http.ResponseWriter, _ *http.Request) {
	s.service.ClearLogs()
	w.Header().Set("Location", "/view/logs")
	w.WriteHeader(303)
}

// handleExportCBZ builds a .cbz archive for one chapter and serves it as a
// file download. The title for the filename is taken from the form.
func (s *Server) handleExportCBZ(w http.ResponseWriter, r *http.Request) {
	pluginID := param(r, "pluginID")
	mangaID := param(r, "mangaID")
	chapterID := param(r, "chapterID")
	if err := r.ParseForm(); err != nil {
		s.logger.Error("export cbz: parse form", "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	title := r.FormValue("title")
	if title == "" {
		title = chapterID
	}
	jobID, err := s.service.EnqueueExportCBZ(r.Context(), pluginID, mangaID, chapterID, title)
	if err != nil {
		s.logger.Error("export cbz", "pluginID", pluginID, "mangaID", mangaID, "chapterID", chapterID, "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]string{"jobID": jobID, "path": ""})
}

// handleJobStatus returns the status of a background job.
func (s *Server) handleJobStatus(w http.ResponseWriter, r *http.Request) {
	jobID := param(r, "jobID")
	status, ok := s.service.GetJobStatus(jobID)
	if !ok {
		http.Error(w, "job not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(status)
}

// handleClearAllCache removes the entire image cache directory.
func (s *Server) handleClearAllCache(w http.ResponseWriter, _ *http.Request) {
	if err := s.service.ClearAllCache(); err != nil {
		s.logger.Error("clear all cache", "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.hxRedirect(w, "/view/settings")
}

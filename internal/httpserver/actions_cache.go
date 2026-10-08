package httpserver

import (
	"encoding/json"
	"mime"
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
	// The client needs the link now, before the job has written the file: the
	// filename is deterministic, so the URL can be handed over immediately and
	// used once the job reports done. Without it the browser only ever saw a
	// filesystem path in a toast and had nothing to click.
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(jobRef{
		Status: "ok",
		JobID:  jobID,
		URL:    s.service.ExportURL(pluginID, mangaID, title),
	})
}

// handleClearAllCache removes the entire image cache directory.
func (s *Server) handleClearAllCache(w http.ResponseWriter, r *http.Request) {
	if err := s.service.ClearAllCache(); err != nil {
		s.logger.Error("clear all cache", "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.hxRedirect(w, r, "/view/settings")
}

// handleDownloadExport serves a finished .cbz. It exists because ExportCBZ writes
// to disk and the enqueue response had no way to point a browser at the result.
func (s *Server) handleDownloadExport(w http.ResponseWriter, r *http.Request) {
	f, info, err := s.service.OpenExport(param(r, "pluginID"), param(r, "mangaID"), r.PathValue("name"))
	if err != nil {
		s.logger.Error("download export", "error", err)
		http.Error(w, "export not found", http.StatusNotFound)
		return
	}
	defer func() { _ = f.Close() }()
	w.Header().Set("Content-Type", "application/vnd.comicbook+zip")
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": info.Name()}))
	http.ServeContent(w, r, info.Name(), info.ModTime(), f)
}

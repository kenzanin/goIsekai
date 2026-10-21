package httpserver

import (
	"net/http"
	"strconv"

	"goisekai/internal/database"
	"goisekai/internal/logger"
)

// apiLibrary mirrors viewLibrary: returns enriched library items with progress.
func (s *Server) apiLibrary(w http.ResponseWriter, r *http.Request) {
	mangas, err := s.service.ListLibrary()
	if err != nil {
		s.logger.Error("api library list", "error", err)
		writeErr(w, http.StatusInternalServerError, "failed to load library")
		return
	}
	libStats, err := s.service.ListLibraryWithProgress()
	if err != nil {
		s.logger.Warn("api library stats", "error", err)
	}
	statsMap := make(map[string]database.LibraryMangaStats, len(libStats))
	for _, st := range libStats {
		statsMap[st.MangaID] = st
	}
	items := make([]apiLibraryItem, 0, len(mangas))
	for _, m := range mangas {
		item := apiLibraryItem{
			Title:         m.Title,
			PluginID:      m.PluginID,
			SourceMangaID: m.SourceMangaID,
			CoverURL:      m.CoverURL,
		}
		if st, ok := statsMap[strconv.FormatInt(m.ID, 10)]; ok {
			item.TotalChapters = st.TotalChapters
			item.ReadChapters = st.ReadChapters
			item.HasNew = st.HasNew
			if st.NewSince != nil {
				ts := st.NewSince.Format("2006-01-02T15:04:05Z")
				item.NewSince = &ts
			}
		}
		items = append(items, item)
	}
	writeJSON(w, http.StatusOK, items)
}

// apiStats returns cache and system statistics for the /api/stats endpoint.
func (s *Server) apiStats(w http.ResponseWriter, r *http.Request) {
	total, hits, err := s.service.CacheStats()
	if err != nil {
		s.logger.Error("api stats", "error", err)
		writeErr(w, http.StatusInternalServerError, "failed to get stats")
		return
	}
	var hitRate float64
	if total > 0 {
		hitRate = float64(hits) / float64(total) * 100
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"total_entries": total,
		"hit_count":     hits,
		"hit_rate":      hitRate,
	})
}

// apiShutdown triggers a graceful shutdown of the server via the AppService.
// The response is written first and the trigger runs asynchronously:
// http.Server.Shutdown waits for in-flight requests, so a synchronous trigger
// would block this handler before it could flush the response — curl then sees
// a connection reset instead of the 200.
func (s *Server) apiShutdown(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "shutting down"})
	go s.service.TriggerShutdown()
}

// apiWorkerLog returns recent worker-job log lines, optionally filtered by
// lane via the ?lane= query parameter. Reads from logger.GetLines (in-memory
// ring buffer). Example: /api/worker-log?lane=interactive
func (s *Server) apiWorkerLog(w http.ResponseWriter, r *http.Request) {
	laneFilter := r.URL.Query().Get("lane")
	lines := logger.GetLines()
	out := make([]string, 0, len(lines))
	if laneFilter == "" {
		out = lines
	} else {
		for _, l := range lines {
			if logger.LaneOf(l) == laneFilter {
				out = append(out, l)
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"lane":  laneFilter,
		"count": len(out),
		"lines": out,
	})
}

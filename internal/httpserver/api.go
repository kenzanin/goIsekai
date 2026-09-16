package httpserver

import (
	"crypto/subtle"
	"github.com/goccy/go-json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"goisekai/internal/database"
)

// writeJSON encodes v as JSON with the given HTTP status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeErr writes a JSON error envelope {"error":"msg"} with the given status.
func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// requireAPIKey returns a middleware that enforces X-API-Key authentication
// when an API key is configured. When the key is empty the middleware is a
// no-op passthrough.
func (s *Server) requireAPIKey(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.apiKey == "" {
			next.ServeHTTP(w, r)
			return
		}
		if subtle.ConstantTimeCompare([]byte(r.Header.Get("X-API-Key")), []byte(s.apiKey)) != 1 {
			writeErr(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// warnIfOpenAPI logs a warning when the server listens on a non-loopback
// address without an API key configured.
func warnIfOpenAPI(logger *slog.Logger, host, apiKey string) {
	if apiKey != "" {
		return
	}
	h := strings.ToLower(host)
	if h == "127.0.0.1" || h == "localhost" || h == "::1" || h == "" {
		return
	}
	logger.Warn("API is exposed without authentication — set -apiKey or api_key in goisekai.ini",
		"host", host)
}

// ---------------------------------------------------------------------------
// API endpoint handlers
// ---------------------------------------------------------------------------

// registerAPIRoutes mounts the JSON API endpoints on r (relative to /api prefix).
func (s *Server) registerAPIRoutes(r chi.Router) {
	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	r.Get("/logs", func(w http.ResponseWriter, _ *http.Request) {
		lines := s.service.GetLogs()
		w.Header().Set("Content-Type", "application/json")
		_ = jsonMarshalLogs(w, lines)
	})
	r.Get("/library", s.apiLibrary)
	r.Get("/library/search", s.apiLibrarySearch)
	r.Get("/search", s.apiSearch)
	r.Delete("/manga/{pluginID}/{mangaID}/alt-titles", s.apiRemoveAltTitle)
	r.Put("/manga/{pluginID}/{mangaID}/title", s.apiSetTitle)
	r.Get("/manga/{pluginID}/{mangaID}", s.apiMangaDetail)
	r.Get("/manga/{pluginID}/{mangaID}/enrichment", s.apiEnrichment)
	r.Get("/manga/{pluginID}/{mangaID}/enrich", s.apiFetchEnrich)
	r.Get("/manga/{pluginID}/{mangaID}/categories", s.apiCategories)
	r.Get("/manga/{pluginID}/{mangaID}/related", s.apiRelated)
	r.Delete("/manga/{pluginID}/{mangaID}/categories", s.apiRemoveCategory)
	r.Delete("/manga/{pluginID}/{mangaID}/related", s.apiRemoveRelated)
	r.Get("/history", s.apiHistory)
	r.Get("/plugins", s.apiPlugins)
	r.Get("/stats", s.apiStats)
	r.Post("/library/toggle/{pluginID}/{mangaID}", s.apiToggleLibrary)
	r.Post("/chapters/read/{pluginID}/{mangaID}/{chapterID}", s.apiMarkChapterRead)
	r.Post("/progress/{pluginID}/{mangaID}/{chapterID}", s.apiSetProgress)
	r.Get("/image/{pluginID}/{mangaID}/{chapterID}", s.apiImage)
	// Thumbnail case: covers have no manga/chapter context — empty cache scope.
	r.Get("/image/{pluginID}", s.apiImage)
}

// apiLibraryItem is the JSON shape for GET /library.
type apiLibraryItem struct {
	Title         string  `json:"title"`
	PluginID      string  `json:"plugin_id"`
	SourceMangaID string  `json:"source_manga_id"`
	CoverURL      string  `json:"cover_url"`
	TotalChapters int     `json:"total_chapters"`
	ReadChapters  int     `json:"read_chapters"`
	HasNew        bool    `json:"has_new"`
	NewSince      *string `json:"new_since"`
}

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

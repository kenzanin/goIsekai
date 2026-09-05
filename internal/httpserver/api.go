package httpserver

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"goisekai/internal/database"
	"goisekai/internal/hostnet"
	"goisekai/pkg/types"
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
	r.Get("/alt-title-servers", s.apiAltTitleServers)
	r.Post("/manga/{pluginID}/{mangaID}/alt-titles", s.apiFetchAltTitles)
	r.Delete("/manga/{pluginID}/{mangaID}/alt-titles", s.apiRemoveAltTitle)
	r.Put("/manga/{pluginID}/{mangaID}/title", s.apiSetTitle)
	r.Get("/manga/{pluginID}/{mangaID}", s.apiMangaDetail)
	r.Get("/history", s.apiHistory)
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
		if st, ok := statsMap[m.ID]; ok {
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

// apiSearchResponse is the JSON shape for GET /search.
type apiSearchResponse struct {
	Results []types.Manga `json:"results"`
	HasNext bool          `json:"has_next"`
	Page    int           `json:"page"`
}

// apiSearch mirrors viewSearch: host-side pagination with search_page_size.
func (s *Server) apiSearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	pluginID := r.URL.Query().Get("pluginID")
	if q == "" || pluginID == "" {
		writeErr(w, http.StatusBadRequest, "missing required query parameters: q, pluginID")
		return
	}
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	results, err := s.service.SearchManga(pluginID, types.SearchFilter{Query: q, Page: page})
	if err != nil {
		if _, ok := errors.AsType[*hostnet.ChallengeError](err); ok {
			writeErr(w, http.StatusForbidden, "source requires verification")
			return
		}
		s.logger.Error("api search", "error", err, "plugin", pluginID, "q", q)
		writeErr(w, http.StatusBadGateway, "search failed: "+err.Error())
		return
	}
	s.service.SyncPluginMeta(pluginID)
	pageSize := s.service.PluginMeta(pluginID).SearchPageSize
	if pageSize <= 0 {
		pageSize = 24
	}
	total := len(results)
	start := min((page-1)*pageSize, total)
	end := min(start+pageSize, total)
	writeJSON(w, http.StatusOK, apiSearchResponse{
		Results: results[start:end],
		HasNext: end < total,
		Page:    page,
	})
}

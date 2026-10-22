package httpserver

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

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
	r.Get("/library/facets", s.apiLibraryFacets)
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
	r.Get("/workers", s.apiWorkers)
	r.Get("/worker-log", s.apiWorkerLog)
	r.Post("/library/toggle/{pluginID}/{mangaID}", s.apiToggleLibrary)
	r.Post("/shutdown", s.apiShutdown)
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
	// Sort keys the desktop client needs; the web handler computes these
	// server-side instead of shipping them.
	UpdatedAt  string `json:"updated_at"`
	LastReadAt string `json:"last_read_at"`
	// DupGroup is the index of the duplicate group this manga belongs to, or -1.
	DupGroup int `json:"dup_group"`
}

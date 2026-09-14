package httpserver

import (
	"crypto/subtle"
	"github.com/goccy/go-json"
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

// apiEnrichmentResponse is the JSON shape for enrichment data.
type apiEnrichmentResponse struct {
	AltTitles    []apiEnrichmentItem `json:"alt_titles"`
	AltSummaries []apiEnrichmentItem `json:"alt_summaries"`
	Categories   []apiEnrichmentItem `json:"categories"`
	Related      []apiEnrichmentItem `json:"related"`
}

// apiEnrichmentItem is one enrichment entry.
type apiEnrichmentItem struct {
	Value  string `json:"value"`
	URL    string `json:"url,omitempty"`
	Source string `json:"source"`
}

// apiEnrichment returns enrichment data for a manga.
func (s *Server) apiEnrichment(w http.ResponseWriter, r *http.Request) {
	pluginID := chi.URLParam(r, "pluginID")
	mangaID := chi.URLParam(r, "mangaID")

	item, err := s.service.GetEnrichment(pluginID, mangaID)
	if err != nil {
		s.logger.Warn("api enrichment", "error", err)
		writeErr(w, http.StatusNotFound, "enrichment not found")
		return
	}

	altTitles := make([]apiEnrichmentItem, 0, len(item.AltTitles))
	for _, a := range item.AltTitles {
		altTitles = append(altTitles, apiEnrichmentItem{Value: a.Value, Source: a.Source})
	}
	altSummaries := make([]apiEnrichmentItem, 0, len(item.AltSummaries))
	for _, a := range item.AltSummaries {
		altSummaries = append(altSummaries, apiEnrichmentItem{Value: a.Value, Source: a.Source})
	}
	categories := make([]apiEnrichmentItem, 0, len(item.Categories))
	for _, c := range item.Categories {
		categories = append(categories, apiEnrichmentItem{Value: c.Value, Source: c.Source})
	}
	related := make([]apiEnrichmentItem, 0, len(item.Related))
	for _, r := range item.Related {
		related = append(related, apiEnrichmentItem{Value: r.Value, URL: r.URL, Source: r.Source})
	}

	writeJSON(w, http.StatusOK, apiEnrichmentResponse{
		AltTitles:    altTitles,
		AltSummaries: altSummaries,
		Categories:   categories,
		Related:      related,
	})
}

// apiCategories returns just the categories for a manga.
func (s *Server) apiCategories(w http.ResponseWriter, r *http.Request) {
	pluginID := chi.URLParam(r, "pluginID")
	mangaID := chi.URLParam(r, "mangaID")

	cats, err := s.service.ListCategories(pluginID, mangaID)
	if err != nil {
		s.logger.Warn("api categories", "error", err)
		writeErr(w, http.StatusInternalServerError, "failed to load categories")
		return
	}

	items := make([]apiEnrichmentItem, 0, len(cats))
	for _, c := range cats {
		items = append(items, apiEnrichmentItem{Value: c.Value, Source: c.Source})
	}
	writeJSON(w, http.StatusOK, items)
}

// apiRelated returns related/recommended manga for a manga.
func (s *Server) apiRelated(w http.ResponseWriter, r *http.Request) {
	pluginID := chi.URLParam(r, "pluginID")
	mangaID := chi.URLParam(r, "mangaID")

	rels, err := s.service.ListRelated(pluginID, mangaID)
	if err != nil {
		s.logger.Warn("api related", "error", err)
		writeErr(w, http.StatusInternalServerError, "failed to load related manga")
		return
	}

	items := make([]apiEnrichmentItem, 0, len(rels))
	for _, r := range rels {
		items = append(items, apiEnrichmentItem{Value: r.Value, URL: r.URL, Source: r.Source})
	}
	writeJSON(w, http.StatusOK, items)
}

// apiRemoveCategory deletes a category from a manga's enrichment data.
func (s *Server) apiRemoveCategory(w http.ResponseWriter, r *http.Request) {
	pluginID := chi.URLParam(r, "pluginID")
	mangaID := chi.URLParam(r, "mangaID")
	q := r.URL.Query()
	category := q.Get("category")
	if category == "" {
		writeErr(w, http.StatusBadRequest, "category not specified")
		return
	}
	if err := s.service.RemoveCategory(pluginID, mangaID, category); err != nil {
		s.logger.Warn("remove category", "error", err)
		writeErr(w, http.StatusInternalServerError, "failed to remove category")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// apiRemoveRelated deletes a related/recommended manga from storage.
func (s *Server) apiRemoveRelated(w http.ResponseWriter, r *http.Request) {
	pluginID := chi.URLParam(r, "pluginID")
	mangaID := chi.URLParam(r, "mangaID")
	q := r.URL.Query()
	title := q.Get("title")
	if title == "" {
		writeErr(w, http.StatusBadRequest, "title not specified")
		return
	}
	if err := s.service.RemoveRelated(pluginID, mangaID, title); err != nil {
		s.logger.Warn("remove related", "error", err)
		writeErr(w, http.StatusInternalServerError, "failed to remove related manga")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// apiFetchEnrich fetches enrichment from a source and returns the updated data
// as JSON. Called by the JS enrichment panel after selecting kind + source.
func (s *Server) apiFetchEnrich(w http.ResponseWriter, r *http.Request) {
	pluginID := chi.URLParam(r, "pluginID")
	mangaID := chi.URLParam(r, "mangaID")
	q := r.URL.Query()
	source := q.Get("source")
	title := q.Get("title")

	if source == "" {
		writeErr(w, http.StatusBadRequest, "source not selected")
		return
	}

	if err := s.service.FetchEnrichment(pluginID, mangaID, title, []string{source}); err != nil {
		s.logger.Error("api fetch enrich", "pluginID", pluginID, "mangaID", mangaID, "source", source, "error", err)
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}

	cats, err := s.service.ListCategories(pluginID, mangaID)
	if err != nil {
		s.logger.Warn("api fetch enrich list categories", "error", err)
	}
	categories := make([]apiEnrichmentItem, 0, len(cats))
	for _, c := range cats {
		categories = append(categories, apiEnrichmentItem{Value: c.Value, Source: c.Source})
	}
	rels, err := s.service.ListRelated(pluginID, mangaID)
	if err != nil {
		s.logger.Warn("api fetch enrich list related", "error", err)
	}
	related := make([]apiEnrichmentItem, 0, len(rels))
	for _, r := range rels {
		related = append(related, apiEnrichmentItem{Value: r.Value, URL: r.URL, Source: r.Source})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"categories": categories,
		"related":    related,
	})
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

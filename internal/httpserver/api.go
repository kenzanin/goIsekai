package httpserver

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
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
	r.Get("/search", s.apiSearch)
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
	Title          string  `json:"title"`
	PluginID       string  `json:"plugin_id"`
	SourceMangaID  string  `json:"source_manga_id"`
	CoverURL       string  `json:"cover_url"`
	TotalChapters  int     `json:"total_chapters"`
	ReadChapters   int     `json:"read_chapters"`
	HasNew         bool    `json:"has_new"`
	NewSince       *string `json:"new_since"`
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
		var ch *hostnet.ChallengeError
		if errors.As(err, &ch) {
			writeErr(w, http.StatusForbidden, "source requires verification")
			return
		}
		s.logger.Error("api search", "error", err, "plugin", pluginID, "q", q)
		writeErr(w, http.StatusBadGateway, "search failed: "+err.Error())
		return
	}
	pageSize := s.service.PluginMeta(pluginID).SearchPageSize
	if pageSize <= 0 {
		pageSize = 24
	}
	total := len(results)
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	writeJSON(w, http.StatusOK, apiSearchResponse{
		Results: results[start:end],
		HasNext: end < total,
		Page:    page,
	})
}

// apiMangaDetailResponse is the JSON shape for GET /manga/{pluginID}/{mangaID}.
type apiMangaDetailResponse struct {
	Title        string            `json:"title"`
	Status       string            `json:"status"`
	Description  string            `json:"description"`
	CoverURL     string            `json:"cover_url"`
	InLibrary    bool              `json:"in_library"`
	Chapters     []apiChapterItem  `json:"chapters"`
	Continue     *apiContinuePoint `json:"continue"`
}

// apiChapterItem is the per-chapter JSON shape inside manga detail.
type apiChapterItem struct {
	ID         string  `json:"id"`
	Title      string  `json:"title"`
	ChapterNum float64 `json:"chapter_num"`
	IsRead     bool    `json:"is_read"`
	LastPage   int     `json:"last_page"`
	TotalPages int     `json:"total_pages"`
}

// apiContinuePoint names where the Continue button should resume.
type apiContinuePoint struct {
	ChapterID string  `json:"chapter_id"`
	ChapterN  float64 `json:"chapter_num"`
	Page      int     `json:"page"`
}

// apiMangaDetail mirrors viewMangaDetail: manga metadata + chapters with progress.
func (s *Server) apiMangaDetail(w http.ResponseWriter, r *http.Request) {
	pluginID := param(r, "pluginID")
	mangaID := param(r, "mangaID")
	// Opening the detail clears the [New] badge.
	_ = s.service.ClearMangaNew(pluginID, mangaID)
	manga, chapters, err := s.service.GetMangaDetails(pluginID, mangaID)
	if err != nil {
		var ch *hostnet.ChallengeError
		if errors.As(err, &ch) {
			writeErr(w, http.StatusForbidden, "source requires verification")
			return
		}
		s.logger.Error("api manga detail", "error", err, "plugin", pluginID, "manga", mangaID)
		writeErr(w, http.StatusNotFound, "manga not found")
		return
	}
	progress, err := s.service.GetChapterProgresses(pluginID, mangaID)
	if err != nil {
		s.logger.Warn("api chapter progress", "error", err)
		progress = map[string]database.ChapterProgress{}
	}
	// Chapters arrive newest-first (desc) — keep that order per D7.
	apiChapters := make([]apiChapterItem, 0, len(chapters))
	for _, c := range chapters {
		ac := apiChapterItem{ID: c.ID, Title: c.Title, ChapterNum: c.ChapterNum}
		if p, ok := progress[c.ID]; ok {
			ac.IsRead = p.Done
			ac.LastPage = p.LastPageRead
			ac.TotalPages = p.TotalPages
		}
		apiChapters = append(apiChapters, ac)
	}
	// Compute continue point.
	continuePt := computeContinueAPI(chapters, progress)
	writeJSON(w, http.StatusOK, apiMangaDetailResponse{
		Title:       manga.Title,
		Status:      manga.Status,
		Description: manga.Description,
		CoverURL:    manga.CoverURL,
		InLibrary:   s.service.IsInLibrary(pluginID, mangaID),
		Chapters:    apiChapters,
		Continue:    continuePt,
	})
}

// computeContinueAPI picks the resume target for the JSON API.
func computeContinueAPI(chapters []types.Chapter, progress map[string]database.ChapterProgress) *apiContinuePoint {
	var firstUnread *apiContinuePoint
	for _, c := range chapters {
		p, ok := progress[c.ID]
		if ok && p.LastPageRead > 0 {
			if p.TotalPages == 0 || p.LastPageRead < p.TotalPages {
				return &apiContinuePoint{ChapterID: c.ID, ChapterN: c.ChapterNum, Page: p.LastPageRead}
			}
			continue
		}
		firstUnread = &apiContinuePoint{ChapterID: c.ID, ChapterN: c.ChapterNum, Page: 1}
	}
	return firstUnread
}

// apiHistoryEntry is the JSON shape for GET /history.
type apiHistoryEntry struct {
	PluginID      string `json:"plugin_id"`
	SourceMangaID string `json:"source_manga_id"`
	Title         string `json:"title"`
	CoverURL      string `json:"cover_url"`
	TotalChapters int    `json:"total_chapters"`
	ReadChapters  int    `json:"read_chapters"`
	LastReadAt    string `json:"last_read_at"`
}

// apiHistory mirrors viewHistory: recently-read manga.
func (s *Server) apiHistory(w http.ResponseWriter, r *http.Request) {
	history, err := s.service.GetReadHistory()
	if err != nil {
		s.logger.Error("api history", "error", err)
		writeErr(w, http.StatusInternalServerError, "failed to load history")
		return
	}
	entries := make([]apiHistoryEntry, 0, len(history))
	for _, h := range history {
		entries = append(entries, apiHistoryEntry{
			PluginID:      h.PluginID,
			SourceMangaID: h.SourceMangaID,
			Title:         h.Title,
			CoverURL:      h.CoverURL,
			TotalChapters: h.TotalChapters,
			ReadChapters:  h.ReadChapters,
			LastReadAt:    h.LastReadAt.Format("2006-01-02T15:04:05Z"),
		})
	}
	writeJSON(w, http.StatusOK, entries)
}

// apiToggleLibrary mirrors handleToggleLibrary: flips in-library and reports state.
func (s *Server) apiToggleLibrary(w http.ResponseWriter, r *http.Request) {
	pluginID := param(r, "pluginID")
	mangaID := param(r, "mangaID")
	if err := s.service.ToggleLibraryItem(pluginID, mangaID); err != nil {
		s.logger.Error("api toggle library", "pluginID", pluginID, "mangaID", mangaID, "error", err)
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{
		"in_library": s.service.IsInLibrary(pluginID, mangaID),
	})
}

// apiMarkChapterReadRequest is the optional POST body for /chapters/read.
type apiMarkChapterReadRequest struct {
	Read *bool `json:"read"`
}

// apiMarkChapterRead mirrors handleMarkChapterRead with optional read:bool body.
func (s *Server) apiMarkChapterRead(w http.ResponseWriter, r *http.Request) {
	pluginID := param(r, "pluginID")
	mangaID := param(r, "mangaID")
	chapterID := param(r, "chapterID")
	readFlag := true
	if r.ContentLength > 0 {
		var req apiMarkChapterReadRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if req.Read != nil {
			readFlag = *req.Read
		}
	}
	if readFlag {
		if err := s.service.MarkChapterRead(pluginID, mangaID, chapterID); err != nil {
			s.logger.Error("api mark read", "error", err)
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
	} else {
		if err := s.service.ResetChapterProgress(pluginID, mangaID, chapterID); err != nil {
			s.logger.Error("api reset read", "error", err)
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]bool{"is_read": readFlag})
}

// apiSetProgressRequest is the POST body for /progress.
type apiSetProgressRequest struct {
	Page int `json:"page"`
}

// apiSetProgress mirrors handleSetChapterProgress: records last-read page.
func (s *Server) apiSetProgress(w http.ResponseWriter, r *http.Request) {
	pluginID := param(r, "pluginID")
	mangaID := param(r, "mangaID")
	chapterID := param(r, "chapterID")
	var req apiSetProgressRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Page < 0 {
		writeErr(w, http.StatusBadRequest, "page must be >= 0")
		return
	}
	if err := s.service.SetChapterProgress(pluginID, mangaID, chapterID, req.Page); err != nil {
		s.logger.Error("api set progress", "error", err)
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"page": req.Page, "total_pages": 0})
}

// apiImage proxies an image through the bridge and returns the raw bytes with
// the detected Content-Type. Mirrors handleImage but uses path params and the
// incoming Referer header instead of query params.
func (s *Server) apiImage(w http.ResponseWriter, r *http.Request) {
	pluginID := param(r, "pluginID")
	mangaID := param(r, "mangaID")
	chapterID := param(r, "chapterID")
	q := r.URL.Query()
	urlStr := q.Get("url")
	if urlStr == "" {
		writeErr(w, http.StatusBadRequest, "missing required query parameter: url")
		return
	}
	if unesc, err := url.QueryUnescape(urlStr); err == nil {
		urlStr = unesc
	}

	var headers map[string]string
	if ref := r.Header.Get("Referer"); ref != "" {
		headers = map[string]string{"Referer": ref}
	}

	s.logger.Debug("api image", "pluginID", pluginID, "url", urlStr, "mangaID", mangaID, "chapterID", chapterID)
	data, err := s.service.GetImage(pluginID, urlStr, headers, mangaID, chapterID)
	if err != nil {
		s.logger.Error("api image fetch", "url", urlStr, "pluginID", pluginID, "error", err)
		writeErr(w, http.StatusBadGateway, "image fetch failed")
		return
	}
	w.Header().Set("Content-Type", http.DetectContentType(data))
	w.Header().Set("Cache-Control", "public, max-age=604800")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

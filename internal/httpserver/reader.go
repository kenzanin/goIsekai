package httpserver

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"goisekai/pkg/types"
)

// registerReaderRoutes mounts the reader page and its JSON data endpoint.
func (s *Server) registerReaderRoutes() {
	s.Router.Get("/view/read/{pluginID}/{mangaID}/{chapterID}", s.viewReader)
	s.Router.Get("/api/reader-data/{pluginID}/{mangaID}/{chapterID}", s.readerData)
}

// chapterNeighbors returns the chapter IDs before/after chapterID in the
// plugin's chapter list (empty string when at either end).
func chapterNeighbors(chapters []types.Chapter, chapterID string) (prev, next string) {
	for i, c := range chapters {
		if c.ID != chapterID {
			continue
		}
		if i > 0 {
			prev = chapters[i-1].ID
		}
		if i+1 < len(chapters) {
			next = chapters[i+1].ID
		}
		return prev, next
	}
	return "", ""
}

// viewReader renders the reader shell; page data is fetched as JSON by the
// inline script from /api/reader-data.
func (s *Server) viewReader(w http.ResponseWriter, r *http.Request) {
	pluginID := chi.URLParam(r, "pluginID")
	mangaID := chi.URLParam(r, "mangaID")
	chapterID := chi.URLParam(r, "chapterID")
	if pluginID == "" || mangaID == "" || chapterID == "" {
		http.Error(w, "missing route params", http.StatusBadRequest)
		return
	}
	manga, chapters, err := s.service.GetMangaDetails(pluginID, mangaID)
	if err != nil {
		s.logger.Error("reader detail", "error", err, "plugin", pluginID, "manga", mangaID)
		http.Error(w, "failed to load manga: "+err.Error(), http.StatusBadGateway)
		return
	}
	prev, next := chapterNeighbors(chapters, chapterID)
	s.renderPage(w, "views/reader.jet", "library", map[string]any{
		"PluginID":      pluginID,
		"MangaID":       mangaID,
		"Manga":         manga,
		"ChapterID":     chapterID,
		"Chapters":      chapters,
		"PrevChapterID": prev,
		"NextChapterID": next,
	})
}

// readerData serves the chapter page list + neighbor chapter IDs as JSON for
// the reader's inline script.
func (s *Server) readerData(w http.ResponseWriter, r *http.Request) {
	pluginID := chi.URLParam(r, "pluginID")
	mangaID := chi.URLParam(r, "mangaID")
	chapterID := chi.URLParam(r, "chapterID")
	if pluginID == "" || mangaID == "" || chapterID == "" {
		http.Error(w, "missing route params", http.StatusBadRequest)
		return
	}
	pages, err := s.service.GetPageList(pluginID, chapterID)
	if err != nil {
		s.logger.Error("reader page list", "error", err, "plugin", pluginID, "chapter", chapterID)
		http.Error(w, "failed to load pages: "+err.Error(), http.StatusBadGateway)
		return
	}
	_, chapters, err := s.service.GetMangaDetails(pluginID, mangaID)
	if err != nil {
		s.logger.Error("reader neighbors", "error", err, "plugin", pluginID, "manga", mangaID)
	}
	prev, next := chapterNeighbors(chapters, chapterID)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"pages":         pages,
		"pluginID":      pluginID,
		"mangaID":       mangaID,
		"chapterID":     chapterID,
		"prevChapterID": prev,
		"nextChapterID": next,
	})
}

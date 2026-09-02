package httpserver

import (
	"encoding/json"
	"net/http"

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
	pluginID := param(r, "pluginID")
	mangaID := param(r, "mangaID")
	chapterID := param(r, "chapterID")
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
	if len(chapters) == 0 {
		chapters, err = s.service.GetChapterList(pluginID, mangaID)
		if err != nil {
			s.logger.Error("reader chapter list fallback", "error", err, "plugin", pluginID, "manga", mangaID)
		}
	}
	prev, next := chapterNeighbors(chapters, chapterID)
	var currentChapter types.Chapter
	for _, c := range chapters {
		if c.ID == chapterID {
			currentChapter = c
			break
		}
	}
	s.renderPage(w, "views/reader.jet", "library", map[string]any{
		"PluginID":       pluginID,
		"MangaID":        mangaID,
		"Manga":          manga,
		"ChapterID":      chapterID,
		"Chapters":       chapters,
		"CurrentChapter": currentChapter,
		"PrevChapterID":  prev,
		"NextChapterID":  next,
	})
}

// readerData serves the chapter page list + neighbor chapter IDs as JSON for
// the reader's inline script.
func (s *Server) readerData(w http.ResponseWriter, r *http.Request) {
	pluginID := param(r, "pluginID")
	mangaID := param(r, "mangaID")
	chapterID := param(r, "chapterID")
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
	// Record the chapter's page count so the detail-page progress badges can
	// render "N/M". Best-effort — a failure here must not fail the read.
	if len(pages) > 0 {
		if err := s.service.SetChapterTotalPages(pluginID, mangaID, chapterID, len(pages)); err != nil {
			s.logger.Warn("record total pages", "error", err, "chapter", chapterID)
		}
	}
	_, chapters, err := s.service.GetMangaDetails(pluginID, mangaID)
	if err != nil {
		s.logger.Error("reader neighbors", "error", err, "plugin", pluginID, "manga", mangaID)
	}
	// If detail response didn't include chapters (e.g. mangzio),
	// fetch the chapter list separately for neighbor resolution.
	if len(chapters) == 0 {
		chapters, err = s.service.GetChapterList(pluginID, mangaID)
		if err != nil {
			s.logger.Error("reader chapter list fallback", "error", err, "plugin", pluginID, "manga", mangaID)
		}
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

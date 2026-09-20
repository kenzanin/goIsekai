package httpserver

import (
	"github.com/goccy/go-json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"goisekai/internal/database"
	"goisekai/pkg/types"
)

// registerReaderRoutes mounts the reader page and its JSON data endpoint.
// r is the router for API routes (under /api prefix).
func (s *Server) registerReaderRoutes(r chi.Router) {
	s.Router.Get("/view/read/{pluginID}/{mangaID}/{chapterID}", s.viewReader)
	r.Get("/reader-data/{pluginID}/{mangaID}/{chapterID}", s.readerData)
}

// chapterNeighbors returns the chapter IDs before/after chapterID in the
// plugin's chapter list (empty string when at either end), skipping any
// chapters the user has marked as skipped.
// chapterNeighbors resolves prev/next in READING order. Chapters arrive
// newest-first (desc), so the older chapter (prev) sits at i+1 and the
// newer chapter (next) at i-1.
func chapterNeighbors(chapters []types.Chapter, progress map[string]database.ChapterProgress, chapterID string) (prev, next string) {
	for i, c := range chapters {
		if c.ID != chapterID {
			continue
		}
		for j := i + 1; j < len(chapters); j++ {
			if progress[chapters[j].ID].IsSkipped {
				continue
			}
			prev = chapters[j].ID
			break
		}
		for j := i - 1; j >= 0; j-- {
			if progress[chapters[j].ID].IsSkipped {
				continue
			}
			next = chapters[j].ID
			break
		}
		return prev, next
	}
	return "", ""
}

// readerChapters resolves the manga and its chapter list for the reader. The
// persisted copy is preferred: it is the same list the detail page shows, and a
// live plugin fetch can come back partial, which would leave navigation with no
// next chapter even though the chapter exists. A manga that was never synced
// falls back to the plugin.
func (s *Server) readerChapters(pluginID, mangaID string) (types.Manga, []types.Chapter, error) {
	manga, chapters, err := s.service.CachedMangaAndChapters(pluginID, mangaID)
	if err == nil && len(chapters) > 0 {
		return manga, chapters, nil
	}

	manga, chapters, err = s.service.GetMangaDetails(pluginID, mangaID)
	if err != nil {
		return manga, chapters, err
	}
	if len(chapters) == 0 {
		live, liveErr := s.service.GetChapterList(pluginID, mangaID)
		if liveErr != nil {
			s.logger.Error("reader chapter list fallback", "error", liveErr, "plugin", pluginID, "manga", mangaID)
			return manga, chapters, nil
		}
		chapters = live
	}
	return manga, chapters, nil
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
	manga, chapters, err := s.readerChapters(pluginID, mangaID)
	if err != nil {
		s.logger.Error("reader detail", "error", err, "plugin", pluginID, "manga", mangaID)
		http.Error(w, "failed to load manga: "+err.Error(), http.StatusBadGateway)
		return
	}
	progress, err := s.service.GetChapterProgresses(pluginID, mangaID)
	if err != nil {
		s.logger.Warn("reader chapter progress", "error", err, "plugin", pluginID, "manga", mangaID)
	}
	prev, next := chapterNeighbors(chapters, progress, chapterID)
	var currentChapter types.Chapter
	for _, c := range chapters {
		if c.ID == chapterID {
			currentChapter = c
			break
		}
	}
	// active "" on purpose: the reader has no nav (blank layout), so a partial
	// response must not advertise one.
	s.renderPage(w, r, "views/reader", "", map[string]any{
		"_layout":        "blank",
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
	pages, err := s.service.GetPageListCached(pluginID, chapterID)
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
	_, chapters, err := s.readerChapters(pluginID, mangaID)
	if err != nil {
		s.logger.Error("reader neighbors", "error", err, "plugin", pluginID, "manga", mangaID)
	}
	progress, err := s.service.GetChapterProgresses(pluginID, mangaID)
	if err != nil {
		s.logger.Warn("reader chapter progress", "error", err, "plugin", pluginID, "manga", mangaID)
	}
	prev, next := chapterNeighbors(chapters, progress, chapterID)
	var chapterNum float64
	var chapterTitle string
	for _, c := range chapters {
		if c.ID == chapterID {
			chapterNum = c.ChapterNum
			chapterTitle = c.Title
			break
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"pages":         pages,
		"pluginID":      pluginID,
		"mangaID":       mangaID,
		"chapterID":     chapterID,
		"prevChapterID": prev,
		"nextChapterID": next,
		"chapterNum":    chapterNum,
		"chapterTitle":  chapterTitle,
	})
}

package httpserver

import (
	"net/http"

	"github.com/goccy/go-json"

	"goisekai/pkg/types"
)

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
	s.logger.Debug("viewReader", "plugin", pluginID, "manga", mangaID, "chapter", chapterID)
	manga, chapters, err := s.readerChapters(pluginID, mangaID)
	if err != nil {
		if s.serviceQueueFull(w, err, "reader") {
			return
		}
		s.logger.Error("reader detail", "error", err, "plugin", pluginID, "manga", mangaID)
		http.Error(w, "failed to load manga: "+err.Error(), http.StatusBadGateway)
		return
	}
	s.logger.Debug("reader chapters loaded", "count", len(chapters), "plugin", pluginID, "manga", mangaID)
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
	// Per-manga reader prefs (JSON blob, "" = global defaults). Best-effort:
	// a miss must not fail the read — the frontend falls back to globals.
	readerPrefs, _, _ := s.service.GetReaderPrefs(pluginID, mangaID)
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
		"ReaderPrefs":    readerPrefs,
	})
}

// readerData serves the chapter page list + neighbor chapter IDs as JSON for
// the reader's inline script.
func (s *Server) readerData(w http.ResponseWriter, r *http.Request) {
	pluginID := param(r, "pluginID")
	mangaID := param(r, "mangaID")
	chapterID := param(r, "chapterID")
	// This is an /api route, so it answers with the JSON error envelope like the
	// rest of them. It used to http.Error plain text, which is why reader.js had
	// to scrape the body: the plugin's reason was there, just not as JSON.
	if pluginID == "" || mangaID == "" || chapterID == "" {
		writeErr(w, http.StatusBadRequest, "missing route params")
		return
	}
	s.logger.Debug("readerData", "plugin", pluginID, "manga", mangaID, "chapter", chapterID)
	pages, err := s.service.GetPageListCached(pluginID, chapterID)
	if err != nil {
		if s.serviceQueueFull(w, err, "reader-pages") {
			return
		}
		s.logger.Error("reader page list", "error", err, "plugin", pluginID, "chapter", chapterID)
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	// If no cached pages (nil = not found), try live fetch from plugin
	if pages == nil {
		pages, err = s.service.GetPageList(pluginID, chapterID)
		if err != nil {
			s.logger.Error("reader page list live fetch", "error", err, "plugin", pluginID, "chapter", chapterID)
			writeErr(w, http.StatusBadGateway, err.Error())
			return
		}
	}
	s.logger.Debug("reader pages fetched", "count", len(pages), "plugin", pluginID, "chapter", chapterID)
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

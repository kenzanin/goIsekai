package httpserver

import (
	"net/http"
	"strconv"
)

// handleSetChapterProgress records the last-read page for a chapter.
func (s *Server) handleSetChapterProgress(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		s.logger.Error("set chapter progress: parse form", "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	pluginID := r.FormValue("pluginID")
	mangaID := r.FormValue("mangaID")
	chapterID := r.FormValue("chapterID")
	page, err := strconv.Atoi(r.FormValue("page"))
	if err != nil || page < 0 {
		s.logger.Error("set chapter progress: bad page", "page", r.FormValue("page"))
		http.Error(w, "invalid 'page' value", http.StatusBadRequest)
		return
	}
	if err := s.service.SetChapterProgress(pluginID, mangaID, chapterID, page); err != nil {
		s.logger.Error("set chapter progress", "pluginID", pluginID, "mangaID", mangaID, "chapterID", chapterID, "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.hxRedirect(w, "/view/manga/"+pluginID+"/"+mangaID)
}

// handleMarkChapterRead marks a single chapter as read.
func (s *Server) handleMarkChapterRead(w http.ResponseWriter, r *http.Request) {
	pluginID := param(r, "pluginID")
	mangaID := param(r, "mangaID")
	chapterID := param(r, "chapterID")
	if err := s.service.MarkChapterRead(pluginID, mangaID, chapterID); err != nil {
		s.logger.Error("mark chapter read", "pluginID", pluginID, "mangaID", mangaID, "chapterID", chapterID, "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.hxRedirect(w, "/view/manga/"+pluginID+"/"+mangaID)
}

// handleResetChapterProgress clears a single chapter's read progress.
func (s *Server) handleResetChapterProgress(w http.ResponseWriter, r *http.Request) {
	pluginID := param(r, "pluginID")
	mangaID := param(r, "mangaID")
	chapterID := param(r, "chapterID")
	if err := s.service.ResetChapterProgress(pluginID, mangaID, chapterID); err != nil {
		s.logger.Error("reset chapter progress", "pluginID", pluginID, "mangaID", mangaID, "chapterID", chapterID, "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.hxRedirect(w, "/view/manga/"+pluginID+"/"+mangaID)
}

// handleChapterActions dispatches the chapter-list action dropdown onto the
// matching bulk progress or cache operation.
func (s *Server) handleChapterActions(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		s.logger.Error("chapter actions: parse form", "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	pluginID := r.FormValue("pluginID")
	mangaID := r.FormValue("mangaID")
	action := r.FormValue("action")
	chapterIDs := r.Form["chapterIDs"]
	if pluginID == "" || mangaID == "" {
		http.Error(w, "missing pluginID or mangaID", http.StatusBadRequest)
		return
	}

	var err error
	switch action {
	case "mark-selected-read", "mark-selected-unread":
		if len(chapterIDs) == 0 {
			http.Error(w, "no chapters selected", http.StatusBadRequest)
			return
		}
		err = s.service.SetChaptersRead(pluginID, mangaID, chapterIDs, action == "mark-selected-read")
	case "mark-up-to", "clear-up-to":
		if len(chapterIDs) == 0 {
			http.Error(w, "no chapters selected", http.StatusBadRequest)
			return
		}
		err = s.service.SetChaptersUpTo(pluginID, mangaID, chapterIDs, action == "mark-up-to")
	case "mark-all-read", "mark-all-unread":
		err = s.service.SetMangaChaptersRead(pluginID, mangaID, action == "mark-all-read")
	case "clear-cache":
		err = s.service.ClearMangaCache(pluginID, mangaID)
	default:
		http.Error(w, "invalid action", http.StatusBadRequest)
		return
	}
	if err != nil {
		s.logger.Error("chapter action", "pluginID", pluginID, "mangaID", mangaID, "action", action, "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.hxRedirect(w, "/view/manga/"+pluginID+"/"+mangaID)
}

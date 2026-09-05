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

// handleMarkChaptersReadBulk marks every chapter listed in the repeated
// chapterIDs form field as read.
func (s *Server) handleMarkChaptersReadBulk(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		s.logger.Error("mark chapters read: parse form", "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	pluginID := r.FormValue("pluginID")
	mangaID := r.FormValue("mangaID")
	ids := r.Form["chapterIDs"]
	if len(ids) == 0 {
		http.Error(w, "no chapters selected", http.StatusBadRequest)
		return
	}
	for _, id := range ids {
		if err := s.service.MarkChapterRead(pluginID, mangaID, id); err != nil {
			s.logger.Error("mark chapter read", "pluginID", pluginID, "mangaID", mangaID, "chapterID", id, "error", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}
	s.hxRedirect(w, "/view/manga/"+pluginID+"/"+mangaID)
}

// handleMarkChapterReadRange marks every chapter from the first referenced id
// up to and including the second, in chapter_num order.
func (s *Server) handleMarkChapterReadRange(w http.ResponseWriter, r *http.Request) {
	pluginID := param(r, "pluginID")
	mangaID := param(r, "mangaID")
	fromID := param(r, "fromID")
	toID := param(r, "toID")
	if err := s.service.MarkChapterReadRange(pluginID, mangaID, fromID, toID); err != nil {
		s.logger.Error("mark chapter read range", "pluginID", pluginID, "mangaID", mangaID, "fromID", fromID, "toID", toID, "error", err)
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

// handleResetMangaProgress clears read progress for every chapter of a manga.
func (s *Server) handleResetMangaProgress(w http.ResponseWriter, r *http.Request) {
	pluginID := param(r, "pluginID")
	mangaID := param(r, "mangaID")
	if err := s.service.ResetMangaProgress(pluginID, mangaID); err != nil {
		s.logger.Error("reset manga progress", "pluginID", pluginID, "mangaID", mangaID, "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.hxRedirect(w, "/view/manga/"+pluginID+"/"+mangaID)
}

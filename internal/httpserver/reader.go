package httpserver

import (
	"fmt"
	"net/http"

	"github.com/goccy/go-json"

	"github.com/go-chi/chi/v5"
)

// registerReaderRoutes mounts the reader page and its JSON data endpoint.
// r is the router for API routes (under /api prefix).
func (s *Server) registerReaderRoutes(r chi.Router) {
	s.Router.Get("/view/read/{pluginID}/{mangaID}/{chapterID}", s.viewReader)
	r.Get("/reader-data/{pluginID}/{mangaID}/{chapterID}", s.readerData)
	r.Post("/reader-warm/{pluginID}/{mangaID}/{chapterID}", s.readerWarm)
	r.Post("/frontend-log", s.handleFrontendLog)
}

// readerWarm takes a read-ahead window and queues it on the server's image
// lane, answering immediately. The reader used to warm these pages itself, one
// /image request per page, which is why read-ahead looked capped: the browser
// holds at most six connections per origin and the displayed page is one of
// them. A window whose pages are not cached yet is a no-op rather than an
// error — the reader has just fetched the chapter's page list, so the empty
// case is a narrow race, not something worth reporting.
func (s *Server) readerWarm(w http.ResponseWriter, r *http.Request) {
	pluginID := param(r, "pluginID")
	mangaID := param(r, "mangaID")
	chapterID := param(r, "chapterID")
	if pluginID == "" || mangaID == "" || chapterID == "" {
		writeErr(w, http.StatusBadRequest, "missing route params")
		return
	}
	var body struct {
		From  int `json:"from"`
		Count int `json:"count"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid payload")
		return
	}
	if body.Count <= 0 {
		writeJSON(w, http.StatusOK, map[string]int{"queued": 0})
		return
	}
	pages, err := s.service.GetPageListCached(pluginID, chapterID)
	if err != nil || pages == nil {
		writeJSON(w, http.StatusOK, map[string]int{"queued": 0})
		return
	}
	from := max(0, min(body.From, len(pages)))
	to := min(from+body.Count, len(pages))
	s.service.WarmPages(pluginID, mangaID, chapterID, pages[from:to])
	writeJSON(w, http.StatusOK, map[string]int{"queued": to - from})
}

// handleFrontendLog receives log messages from the frontend and writes them to the server log.
// This allows tracking JS runtime behavior in the application logs alongside Go logs.
func (s *Server) handleFrontendLog(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Tag   string `json:"tag"`   // e.g. "reader", "ui", "network"
		Level string `json:"level"` // "log", "warn", "error"
		Msg   string `json:"msg"`
		SID   string `json:"sid"` // session/correlation ID
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid payload")
		return
	}
	tag := payload.Tag
	if tag == "" {
		tag = "frontend"
	}
	// Fire-and-forget log — don't block the client. Plain client chatter is
	// Debug: reader.js emits a line per rendered page, which would otherwise
	// bury the server's own INFO records. Warnings and errors keep their level.
	msg := fmt.Sprintf("[%s] %s", tag, payload.Msg)
	switch payload.Level {
	case "warn":
		s.logger.Warn(msg, "sid", payload.SID)
	case "error":
		s.logger.Error(msg, "sid", payload.SID)
	default:
		s.logger.Debug(msg, "sid", payload.SID)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNoContent)
}

package httpserver

import (
	"time"

	"github.com/go-chi/chi/v5"
	"golang.org/x/net/websocket"
)

// registerWSRoutes mounts the live log stream endpoint on the given router.
func (s *Server) registerWSRoutes(r chi.Router) {
	r.Handle("/logs/ws", websocket.Handler(s.streamLogs))
}

// streamLogs pushes every buffered line, then follows the ring buffer for new
// entries until the client disconnects.
func (s *Server) streamLogs(ws *websocket.Conn) {
	defer ws.Close()
	sent := 0
	buf := make([]byte, 0, 4096)
	for {
		for _, line := range s.service.GetLogs()[sent:] {
			buf = append(buf[:0], line...)
			if _, err := ws.Write(buf); err != nil {
				return
			}
			sent++
		}
		// ponytail: 250ms poll of the ring buffer; swap for a pub-sub channel
		// when multiple clients or high log volume matter.
		time.Sleep(250 * time.Millisecond)
	}
}

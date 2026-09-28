package httpserver

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"golang.org/x/net/websocket"
	"goisekai/internal/workers"
)

// JobStatusWSMessage is the JSON message format for job status WS updates.
type JobStatusWSMessage struct {
	JobID   string `json:"jobID"`
	Status  string `json:"status"`
	Attempt int    `json:"attempt,omitempty"`
}

// registerWSRoutes mounts the live log stream endpoint on the given router.
func (s *Server) registerWSRoutes(r chi.Router) {
	r.Handle("/logs/ws", websocket.Handler(s.streamLogs))
	r.Handle("/jobs/ws", websocket.Handler(s.streamJobs))
	// Wire the pool's StatusChanged callback for job broadcasts.
	if pool := s.service.GetPool(); pool != nil {
		pool.StatusChanged = func(id string, status workers.JobStatus) {
			s.broadcastJobStatus(id, status.String())
		}
	}
}

// streamLogs pushes every buffered line, then follows the ring buffer for new
// entries until the client disconnects.
func (s *Server) streamLogs(ws *websocket.Conn) {
	defer func() { _ = ws.Close() }()
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

// jobClients tracks WebSocket connections interested in job status updates.
// ponytail: simple broadcast, upgrade to per-connection pub-sub when scaling.
type jobClients struct {
	mu       sync.Mutex
	conns    map[*websocket.Conn]struct{}
	filtered map[string]map[*websocket.Conn]struct{} // jobID -> connections with filter
}

var clients = &jobClients{
	conns:    make(map[*websocket.Conn]struct{}),
	filtered: make(map[string]map[*websocket.Conn]struct{}),
}

// streamJobs maintains a WS connection for job status updates.
// Clients can send {"jobID": "..."} to filter for a specific job.
func (s *Server) streamJobs(ws *websocket.Conn) {
	defer func() {
		s.unregisterJobClient(ws)
		_ = ws.Close()
	}()

	// Register unfiltered client for all job updates
	clients.mu.Lock()
	clients.conns[ws] = struct{}{}
	clients.mu.Unlock()

	readDone := make(chan struct{})
	go func() {
		for {
			var msg struct{ JobID string }
			if err := json.NewDecoder(ws).Decode(&msg); err != nil {
				close(readDone)
				return
			}
			clients.filterClient(ws, msg.JobID)
		}
	}()

	<-readDone

	// Keep connection open for broadcasts
	for {
		time.Sleep(time.Second)
	}
}

func (s *Server) unregisterJobClient(ws *websocket.Conn) {
	clients.mu.Lock()
	delete(clients.conns, ws)
	for _, conns := range clients.filtered {
		delete(conns, ws)
	}
	clients.mu.Unlock()
}

func (c *jobClients) filterClient(ws *websocket.Conn, jobID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if jobID == "" {
		// No filter - stay in unfiltered
		return
	}
	if c.filtered[jobID] == nil {
		c.filtered[jobID] = make(map[*websocket.Conn]struct{})
	}
	c.filtered[jobID][ws] = struct{}{}
	delete(c.conns, ws)
}

// broadcastJobStatus is called by the pool when job status changes.
func (s *Server) broadcastJobStatus(id string, statusStr string) {
	clients.mu.Lock()
	defer clients.mu.Unlock()

	msg, _ := json.Marshal(JobStatusWSMessage{JobID: id, Status: statusStr})

	// Send to filtered connections
	if conns, ok := clients.filtered[id]; ok {
		for ws := range conns {
			_, _ = ws.Write(msg)
		}
	}
	// Send to unfiltered connections
	for ws := range clients.conns {
		_, _ = ws.Write(msg)
	}
}
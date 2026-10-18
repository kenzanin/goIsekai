package httpserver

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"goisekai/internal/workers"
	"golang.org/x/net/websocket"
)

// JobStatusWSMessage is the JSON message format for job status WS updates.
// Field names match design.md's contract: job_id (snake_case), status, and an
// optional path carried only on done for export jobs. Status "progress" is an
// in-run heartbeat carrying progress (0-100) instead of a lifecycle change.
type JobStatusWSMessage struct {
	JobID    string `json:"job_id"`
	Status   string `json:"status"`
	Path     string `json:"path,omitempty"`
	Attempt  int    `json:"attempt,omitempty"`
	Progress int    `json:"progress,omitempty"`
}

// registerWSRoutes mounts the live log stream endpoint on the given router.
func (s *Server) registerWSRoutes(r chi.Router) {
	r.Handle("/logs/ws", websocket.Handler(s.streamLogs))
	r.Handle("/jobs/ws", websocket.Handler(s.streamJobs))
	// Wire the pool's StatusChanged callback for job broadcasts.
	if pool := s.service.GetPool(); pool != nil {
		pool.StatusChanged = func(id string, status workers.JobStatus, detail string) {
			s.broadcastJobStatus(id, status.String(), detail)
		}
		pool.ProgressChanged = func(id string, pct int) {
			s.broadcastJobProgress(id, pct)
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
// Clients can send {"job_id": "..."} to filter for a specific job.
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
			var msg struct {
				JobID string `json:"job_id"`
			}
			if err := json.NewDecoder(ws).Decode(&msg); err != nil {
				close(readDone)
				return
			}
			clients.filterClient(ws, msg.JobID)
		}
	}()

	<-readDone
	// Client disconnected (Decode returned EOF or error). Loop until the
	// broadcast channel has no more writers / connection is torn down below.
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
		// Un-filter: remove from all filtered buckets and restore to unfiltered.
		for _, conns := range c.filtered {
			delete(conns, ws)
		}
		c.conns[ws] = struct{}{}
		return
	}
	if c.filtered[jobID] == nil {
		c.filtered[jobID] = make(map[*websocket.Conn]struct{})
	}
	c.filtered[jobID][ws] = struct{}{}
	delete(c.conns, ws)
}

// broadcastJobStatus is called by the pool when job status changes.
func (s *Server) broadcastJobStatus(id string, statusStr string, path string) {
	s.broadcastJobMessage(JobStatusWSMessage{JobID: id, Status: statusStr, Path: path})
}

// broadcastJobProgress is called by the pool when a running job reports
// incremental progress (0-100).
func (s *Server) broadcastJobProgress(id string, pct int) {
	s.broadcastJobMessage(JobStatusWSMessage{JobID: id, Status: "progress", Progress: pct})
}

func (s *Server) broadcastJobMessage(msg JobStatusWSMessage) {
	clients.mu.Lock()
	defer clients.mu.Unlock()
	payload, _ := json.Marshal(msg)

	// Send to filtered connections
	if conns, ok := clients.filtered[msg.JobID]; ok {
		for ws := range conns {
			_, _ = ws.Write(payload)
		}
	}
	// Send to unfiltered connections
	for ws := range clients.conns {
		_, _ = ws.Write(payload)
	}
}

package httpserver

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"goisekai/internal/workers"

	"golang.org/x/net/websocket"
)

// wsURL converts an httptest http:// URL to ws://.../api/jobs/ws.
func wsURL(httpURL string) string {
	u := httpURL
	if len(u) >= 8 && u[:7] == "http://" {
		u = "ws" + u[4:]
	}
	return u + "/api/jobs/ws"
}

// wsDial wraps websocket.Dial with a valid origin string (empty origin is rejected).
func wsDial(rawurl string) (*websocket.Conn, error) {
	return websocket.Dial(rawurl, "", "http://localhost")
}

// TestJobsWSBroadcast verifies a pool status push reaches a WS subscriber, and
// that filter→unfilter cycles keep the socket receiving (regression for the
// filterClient un-filter leak that orphaned sockets, and streamJobs infinite
// Sleep leak). Uses ONE Server/pool instance to avoid global-client-map races.
func TestJobsWSBroadcast(t *testing.T) {
	s := testServerFull(t, "", true)
	if s.service == nil || s.service.GetPool() == nil {
		t.Skip("service/worker pool not available")
	}
	pool := s.service.GetPool()

	httpSrv := httptest.NewServer(s.Router)
	defer httpSrv.Close()

	u := httpSrv.URL
	if len(u) >= 8 && u[:7] == "http://" {
		u = "ws" + u[4:]
	}
	conn, err := wsDial(u + "/api/jobs/ws")
	if err != nil {
		t.Fatalf("ws dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	// filter then un-filter
	send := func(id string) {
		_ = json.NewEncoder(conn).Encode(struct {
			JobID string `json:"job_id"`
		}{JobID: id})
	}
	send("job-123")
	send("")

	// Drive the SAME callback production wires (registerWSRoutes sets pool.StatusChanged).
	pool.StatusChanged("job-123", workers.StatusRunning, "")
	pool.StatusChanged("job-123", workers.StatusDone, "/out/x.cbz")

	timeout := time.After(2 * time.Second)
	got := make(chan JobStatusWSMessage, 8)
	go func() {
		for {
			var msg JobStatusWSMessage
			if err := json.NewDecoder(conn).Decode(&msg); err != nil {
				close(got)
				return
			}
			select {
			case got <- msg:
			case <-timeout:
				return
			}
		}
	}()

	var seenDone bool
	for !seenDone {
		select {
		case <-timeout:
			t.Fatal("timed out waiting for job-123 WS message")
		case msg := <-got:
			if msg.JobID != "job-123" {
				t.Fatalf("expected job-123, got %s", msg.JobID)
			}
			if msg.Status == workers.StatusDone.String() {
				seenDone = true
				if msg.Path != "/out/x.cbz" {
					t.Errorf("path = %q, want /out/x.cbz", msg.Path)
				}
			}
		}
	}
}

// TestJobsWSProgress verifies pool.SetProgress flows through the wired
// ProgressChanged callback to WS subscribers as a "progress" message.
func TestJobsWSProgress(t *testing.T) {
	s := testServerFull(t, "", true)
	if s.service == nil || s.service.GetPool() == nil {
		t.Skip("service/worker pool not available")
	}
	pool := s.service.GetPool()

	httpSrv := httptest.NewServer(s.Router)
	defer httpSrv.Close()

	conn, err := wsDial(wsURL(httpSrv.URL))
	if err != nil {
		t.Fatalf("ws dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	// Drive the SAME callback production wires (registerWSRoutes sets pool.ProgressChanged).
	pool.SetProgress("job-77", 42)

	deadline := time.Now().Add(2 * time.Second)
	for {
		if err := conn.SetReadDeadline(deadline); err != nil {
			t.Fatalf("set read deadline: %v", err)
		}
		var msg JobStatusWSMessage
		if err := json.NewDecoder(conn).Decode(&msg); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if msg.JobID != "job-77" {
			continue
		}
		if msg.Status != "progress" {
			t.Fatalf("status = %q, want progress", msg.Status)
		}
		if msg.Progress != 42 {
			t.Fatalf("progress = %d, want 42", msg.Progress)
		}
		return
	}
}

// TestJobsWSUnregister verifies a disconnecting client is removed from the
// fan-out without panic on the next broadcast.
func TestJobsWSUnregister(t *testing.T) {
	s := testServerFull(t, "", true)
	httpSrv := httptest.NewServer(s.Router)
	defer httpSrv.Close()

	conn, err := wsDial(wsURL(httpSrv.URL))
	if err != nil {
		t.Fatalf("ws dial: %v", err)
	}
	_ = conn.Close()

	s.broadcastJobStatus("job-999", workers.StatusDone.String(), "")
	time.Sleep(50 * time.Millisecond)
}

// TestJobsWSRespectsAPIKey ensures WS is gated behind the API-key middleware.
func TestJobsWSRespectsAPIKey(t *testing.T) {
	s := testServer(t, "secret")
	httpSrv := httptest.NewServer(s.Router)
	defer httpSrv.Close()

	_, err := wsDial(wsURL(httpSrv.URL))
	if err == nil {
		t.Fatal("expected auth failure without API key")
	}
}

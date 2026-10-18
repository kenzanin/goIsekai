package httpserver

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/goccy/go-json"

	"goisekai/internal/logger"
	"goisekai/internal/workers"
)

// seedLaneLines installs the capture handler and writes one worker record per
// lane plus a lane-less line. viewLogs and apiWorkerLog both read the global
// ring buffer, so the filter can only be exercised against a live buffer.
func seedLaneLines(t *testing.T) {
	t.Helper()
	if err := logger.Init("debug"); err != nil {
		t.Fatalf("logger.Init: %v", err)
	}
	logger.Clear()
	t.Cleanup(logger.Clear)

	logger.Debug("worker: running", "jobID", "fetch-1", "lane", workers.LaneFetch.String(), "plugin", "mangataro")
	logger.Debug("worker: running", "jobID", "image-1", "lane", workers.LaneImage.String())
	logger.Info("starting goIsekai", "log_level", "debug")
}

// TestViewLogsLaneFilter is the server half of the lane dropdown: asking for
// one lane must keep only the records tagged with it. This is exactly what
// broke — the filter looked for "lane= <Name>" and the lines carry "lane=fetch".
func TestViewLogsLaneFilter(t *testing.T) {
	seedLaneLines(t)
	s := testServerFull(t, "", true)

	req := httptest.NewRequest("GET", "/view/logs?lane="+workers.LaneFetch.String(), nil)
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "jobID=fetch-1") {
		t.Error("fetch line missing from a fetch-lane view")
	}
	if strings.Contains(body, "jobID=image-1") {
		t.Error("image line leaked into a fetch-lane view")
	}
}

// TestViewLogsLaneFilterAllKeepsEverything: no lane param means no filtering.
func TestViewLogsLaneFilterAllKeepsEverything(t *testing.T) {
	seedLaneLines(t)
	s := testServerFull(t, "", true)

	req := httptest.NewRequest("GET", "/view/logs", nil)
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)
	body := rec.Body.String()
	if !strings.Contains(body, "jobID=fetch-1") || !strings.Contains(body, "jobID=image-1") {
		t.Errorf("unfiltered view dropped lines; got %d bytes", len(body))
	}
}

// TestAPIWorkerLogLaneFilter pins the JSON endpoint to the same parser as the
// view, so the two cannot drift onto different lane shapes.
func TestAPIWorkerLogLaneFilter(t *testing.T) {
	seedLaneLines(t)
	s := testServer(t, "")

	req := httptest.NewRequest("GET", "/api/worker-log?lane="+workers.LaneImage.String(), nil)
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body struct {
		Lane  string   `json:"lane"`
		Count int      `json:"count"`
		Lines []string `json:"lines"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(body.Lines) != 1 || !strings.Contains(body.Lines[0], "jobID=image-1") {
		t.Fatalf("lane=%s returned %v, want just the image line", body.Lane, body.Lines)
	}
}

// TestLaneNamesMatchUI pins the lane vocabulary the log page's filter offers.
// The JS laneMap in views/logs.lua hard-codes these same lower-case values, so
// a change to workers.Lane.String() must be mirrored in that map. The dropdown
// values are now built from this vocabulary; this test is the Go-side guard.
func TestLaneNamesMatchUI(t *testing.T) {
	want := []string{"interactive", "fetch", "image", "maintenance", "cdp"}
	lanes := []workers.Lane{
		workers.LaneInteractive, workers.LaneFetch, workers.LaneImage,
		workers.LaneMaintenance, workers.LaneCDP,
	}
	if len(lanes) != len(want) {
		t.Fatalf("lane count = %d, want %d", len(lanes), len(want))
	}
	for i, lane := range lanes {
		if got := lane.String(); got != want[i] {
			t.Errorf("lane %d = %q, want %q (views/logs.lua laneMap must match)", i, got, want[i])
		}
	}
}

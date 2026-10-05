package httpserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"goisekai/internal/workers"
)

type apiWorkersLane struct {
	Lane      string `json:"lane"`
	Waiting   int    `json:"waiting"`
	Running   int    `json:"running"`
	Capacity  int    `json:"capacity"`
	Completed int64  `json:"completed"`
	Failed    int64  `json:"failed"`
	Abandoned int64  `json:"abandoned"`

	Samples      int   `json:"duration_samples"`
	MedianMS     int64 `json:"median_ms"`
	P99MS        int64 `json:"p99_ms"`
	HasDurations bool  `json:"has_durations"`
}

// Task 5.1: lane state is reachable through the host's own read interface — no
// metrics collector or extra process — and every lane is present with a capacity.
func TestAPIWorkersReportsAllLanes(t *testing.T) {
	s := testServer(t, "")
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/workers", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	var out struct {
		Lanes []apiWorkersLane `json:"lanes"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v (body: %s)", err, rec.Body.String())
	}
	if len(out.Lanes) != 4 {
		t.Fatalf("lanes = %d, want 4 (body: %s)", len(out.Lanes), rec.Body.String())
	}

	seen := map[string]bool{}
	for _, l := range out.Lanes {
		switch l.Lane {
		case workers.LaneInteractive.String(), workers.LaneFetch.String(),
			workers.LaneImage.String(), workers.LaneMaintenance.String():
		default:
			t.Errorf("unexpected lane %q", l.Lane)
			continue
		}
		if seen[l.Lane] {
			t.Errorf("lane %s reported twice", l.Lane)
		}
		seen[l.Lane] = true
		if l.Capacity <= 0 {
			t.Errorf("lane %s capacity = %d, want > 0", l.Lane, l.Capacity)
		}
		if l.Waiting < 0 || l.Running < 0 {
			t.Errorf("lane %s has negative counts: waiting=%d running=%d", l.Lane, l.Waiting, l.Running)
		}
		// A lane that has never run a job reports "no durations" rather than a
		// zero duration that would read as instantaneous work.
		if !l.HasDurations && (l.Samples != 0 || l.MedianMS != 0 || l.P99MS != 0) {
			t.Errorf("lane %s reports durations %d/%dms/%dms with has_durations=false",
				l.Lane, l.Samples, l.MedianMS, l.P99MS)
		}
	}
	for _, lane := range []string{
		workers.LaneInteractive.String(), workers.LaneFetch.String(),
		workers.LaneImage.String(), workers.LaneMaintenance.String(),
	} {
		if !seen[lane] {
			t.Errorf("lane %s missing from the response", lane)
		}
	}
}

// The endpoint lives under /api, so the API-key gate applies like every other
// JSON route.
func TestAPIWorkersHonoursAPIKey(t *testing.T) {
	s := testServer(t, "secret")

	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/workers", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("without key: status = %d, want 401", rec.Code)
	}

	rec = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/workers", nil)
	req.Header.Set("X-API-Key", "secret")
	s.Router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("with key: status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
}

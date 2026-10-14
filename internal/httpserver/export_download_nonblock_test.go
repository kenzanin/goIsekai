package httpserver

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"goisekai/internal/workers"
)

// TestExportCBZDoesNotBlockHandler is the regression guard for task 2.4:
// a CBZ export must queue on the image lane and the HTTP handler must reply
// with a job ref immediately — it must never block until ExportCBZ finishes.
//
// We occupy one image-worker with a long-running job; the export enqueues on
// a second worker, so the handler returns ~instantly. If handleExportCBZ ever
// regressed to awaiting the export synchronously, this test would block ~10s.
func TestExportCBZDoesNotBlockHandler(t *testing.T) {
	s := testServerFull(t, "", true)
	pool := s.service.GetPool()
	if pool == nil {
		t.Fatal("service pool is nil")
	}

	// Occupy an image worker with a 10s job on the HIGH queue.
	_, err := pool.Enqueue(context.Background(), &workers.Job{
		Lane:        workers.LaneImage,
		Priority:    workers.PriorityHigh,
		MaxAttempts: 1,
		Run: func(context.Context) error {
			time.Sleep(10 * time.Second)
			return nil
		},
	})
	if err != nil {
		t.Fatalf("prime image worker: %v", err)
	}
	// Give the worker a moment to pick the job up so the export lands on a
	// different (idle) worker rather than queuing behind the slow one.
	time.Sleep(50 * time.Millisecond)

	req := csrfPost(s, "/action/export-cbz/dummy/manga1/chapter1", strings.NewReader("title=Ch.-1"))
	rec := httptest.NewRecorder()

	start := time.Now()
	s.Router.ServeHTTP(rec, req)
	elapsed := time.Since(start)

	if rec.Code >= 400 {
		t.Fatalf("export-cbz enqueue returned %d: %s", rec.Code, rec.Body.String())
	}
	// Hard regression boundary: export job sleeps 10s; handler must reply
	// well under that. If it regressed to awaiting the export this would
	// be >=10s.
	if elapsed >= 2*time.Second {
		t.Fatalf("handler blocked for %s (want < 2s) — export-cbz is blocking the request goroutine", elapsed)
	}

	var ref jobRef
	if err := json.Unmarshal(rec.Body.Bytes(), &ref); err != nil {
		t.Fatalf("decode jobRef: %v (body %s)", err, rec.Body.String())
	}
	if ref.JobID == "" {
		t.Fatal("no job_id in response")
	}
	if !strings.HasPrefix(ref.URL, "/exports/dummy/manga1/") || !strings.HasSuffix(ref.URL, ".cbz") {
		t.Errorf("url = %q, want /exports/dummy/manga1/*.cbz", ref.URL)
	}
}

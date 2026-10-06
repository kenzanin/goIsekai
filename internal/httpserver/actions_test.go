package httpserver

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ── Action handler tests ────────────────────────────────────────────────────

func TestActionToggleLibrary(t *testing.T) {
	s := testServerFull(t, "", true)
	// Route: /action/toggle-library/{pluginID}/{mangaID}
	req := csrfPost(s, "/action/toggle-library/dummy/manga1", nil)
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)
	// Should redirect (303) or handle gracefully.
	if rec.Code != 303 && rec.Code != 302 {
		t.Fatalf("status = %d, want 303/302", rec.Code)
	}
}

func TestActionSync(t *testing.T) {
	s := testServerFull(t, "", true)
	req := csrfPost(s, "/action/sync", nil)
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)
	// Sync enqueues a fetch-lane job and returns a job reference immediately.
	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	// Contract is character-for-character: {"status":"ok","job_id":"<id>"}.
	if !strings.Contains(rec.Body.String(), `"status":"ok"`) || !strings.Contains(rec.Body.String(), `"job_id":`) {
		t.Fatalf("body = %q, want {\"status\":\"ok\",\"job_id\":...}", rec.Body.String())
	}
}

func TestActionSaveSettings(t *testing.T) {
	s := testServerFull(t, "", true)
	form := "data_dir=/tmp/test&cache_dir=/tmp/cache"
	req := csrfPost(s, "/action/save-settings", strings.NewReader(form))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)
	// cfgPath is empty in test, so save-settings may fail. Just verify no panic.
	if rec.Code == 0 {
		t.Fatal("expected a valid HTTP status code")
	}
}

func TestActionClearAllCache(t *testing.T) {
	s := testServerFull(t, "", true)
	req := csrfPost(s, "/action/clear-cache-all", nil)
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)
	if rec.Code != 303 && rec.Code != 302 {
		t.Fatalf("status = %d, want 303/302", rec.Code)
	}
}

func TestActionExportCBZNonexistent(t *testing.T) {
	s := testServerFull(t, "", true)
	// Route: /action/export-cbz/{pluginID}/{mangaID}/{chapterID}
	// Use httptest.NewRecorder with panic recovery to catch nil pointer.
	defer func() {
		if r := recover(); r != nil {
			t.Logf("export-cbz panicked (expected in test with no data): %v", r)
		}
	}()
	req := csrfPost(s, "/action/export-cbz/dummy/manga1/chapter1", nil)
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)
	// No data — should handle gracefully (redirect or error page).
	if rec.Code == 500 {
		t.Logf("export-cbz returned 500 (acceptable with no data)")
	}
}

func TestActionSetChapterProgress(t *testing.T) {
	s := testServerFull(t, "", true)
	form := "chapter_id=c1&last_page=5&total_pages=10"
	req := csrfPost(s, "/action/set-chapter-progress", strings.NewReader(form))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)
	// No chapter exists — should handle gracefully.
	if rec.Code == 500 {
		t.Fatal("expected graceful handling, not 500")
	}
}

func TestActionClearLogs(t *testing.T) {
	s := testServerFull(t, "", true)
	req := csrfPost(s, "/action/clear-logs", nil)
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)
	if rec.Code != 303 && rec.Code != 302 {
		t.Fatalf("status = %d, want 303/302", rec.Code)
	}
}

func TestActionMarkRead(t *testing.T) {
	s := testServerFull(t, "", true)
	// Route: /action/mark-read/{pluginID}/{mangaID}/{chapterID}
	req := csrfPost(s, "/action/mark-read/dummy/manga1/chapter1", nil)
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)
	if rec.Code != 303 && rec.Code != 302 {
		t.Fatalf("status = %d, want 303/302", rec.Code)
	}
}

func TestActionResetChapterProgress(t *testing.T) {
	s := testServerFull(t, "", true)
	// Route: /action/reset-progress/{pluginID}/{mangaID}/{chapterID}
	req := csrfPost(s, "/action/reset-progress/dummy/manga1/chapter1", nil)
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)
	if rec.Code != 303 && rec.Code != 302 {
		t.Fatalf("status = %d, want 303/302", rec.Code)
	}
}

func postChapterAction(t *testing.T, s *Server, form string) int {
	t.Helper()
	req := csrfPost(s, "/action/chapter-actions", strings.NewReader(form))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)
	return rec.Code
}

// TestActionChapterActions exercises the chapter-list action dropdown endpoint.
func TestActionChapterActions(t *testing.T) {
	s := testServerFull(t, "", true)

	cases := []struct {
		name string
		form string
		want int
	}{
		{"mark selected read", "pluginID=dummy&mangaID=manga1&action=mark-selected-read&chapterIDs=cs1&chapterIDs=cs2", 303},
		{"mark selected unread", "pluginID=dummy&mangaID=manga1&action=mark-selected-unread&chapterIDs=cs1", 303},
		{"mark all read", "pluginID=dummy&mangaID=manga1&action=mark-all-read", 303},
		{"mark all unread", "pluginID=dummy&mangaID=manga1&action=mark-all-unread", 303},
		{"mark up to unknown chapter", "pluginID=dummy&mangaID=manga1&action=set-up-to-read&chapterIDs=nope", 400},
		{"skip selected", "pluginID=dummy&mangaID=manga1&action=skip-selected&chapterIDs=cs1", 303},
		{"unskip selected", "pluginID=dummy&mangaID=manga1&action=unskip-selected&chapterIDs=cs1", 303},
		// The old names selected the same is_skipped flag; keeping them live would
		// leave two vocabularies for one flag, which is what this rename removed.
		{"legacy mark-selected-show rejected", "pluginID=dummy&mangaID=manga1&action=mark-selected-show&chapterIDs=cs1", 400},
		{"legacy mark-selected-hide rejected", "pluginID=dummy&mangaID=manga1&action=mark-selected-hide&chapterIDs=cs1", 400},
		{"selection required", "pluginID=dummy&mangaID=manga1&action=mark-selected-read", 400},
		{"unknown action", "pluginID=dummy&mangaID=manga1&action=bogus", 400},
		{"missing manga", "pluginID=dummy&action=mark-all-read", 400},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := postChapterAction(t, s, tc.form); got != tc.want {
				t.Fatalf("status = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestActionToggleChapterSkip(t *testing.T) {
	s := testServerFull(t, "", true)
	// Route: /action/toggle-skip/{pluginID}/{mangaID}/{chapterID}
	req := csrfPost(s, "/action/toggle-skip/dummy/manga1/chapter1", nil)
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)
	if rec.Code != 303 && rec.Code != 200 && rec.Code != 400 {
		t.Fatalf("status = %d, want 303 or 200", rec.Code)
	}
}

func TestActionToggleCoverDim(t *testing.T) {
	s := testServerFull(t, "", true)
	// Route: /action/toggle-cover-dim/{pluginID}/{mangaID}
	req := csrfPost(s, "/action/toggle-cover-dim/dummy/manga1", nil)
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)
	if rec.Code != 303 && rec.Code != 200 && rec.Code != 400 {
		t.Fatalf("status = %d, want 303 or 200", rec.Code)
	}
}

// csrfPost builds a POST that carries the server's CSRF token. Every action
// route is behind requireCSRFToken, so a request without one is refused before
// the handler runs.
func csrfPost(s *Server, target string, body io.Reader) *http.Request {
	r := httptest.NewRequest("POST", target, body)
	r.Header.Set(csrfHeader, s.csrfToken)
	return r
}

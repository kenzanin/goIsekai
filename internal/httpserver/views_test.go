package httpserver

import (
	"net/http/httptest"
	"testing"
)

// ── View handler tests ──────────────────────────────────────────────────────

func TestViewLibrary(t *testing.T) {
	s := testServerFull(t, "", true)
	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
}

func TestViewLibraryRoute(t *testing.T) {
	s := testServerFull(t, "", true)
	req := httptest.NewRequest("GET", "/view/library", nil)
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestViewHistory(t *testing.T) {
	s := testServerFull(t, "", true)
	req := httptest.NewRequest("GET", "/view/history", nil)
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestViewSearchNoQuery(t *testing.T) {
	s := testServerFull(t, "", true)
	req := httptest.NewRequest("GET", "/view/search", nil)
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestViewSearchWithQuery(t *testing.T) {
	s := testServerFull(t, "", true)
	req := httptest.NewRequest("GET", "/view/search?q=naruto&plugin=dummy", nil)
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestViewSettings(t *testing.T) {
	s := testServerFull(t, "", true)
	req := httptest.NewRequest("GET", "/view/settings", nil)
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestViewPlugins(t *testing.T) {
	s := testServerFull(t, "", true)
	req := httptest.NewRequest("GET", "/view/plugins", nil)
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestViewLogs(t *testing.T) {
	s := testServerFull(t, "", true)
	req := httptest.NewRequest("GET", "/view/logs", nil)
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestViewMangaDetailNonexistent(t *testing.T) {
	s := testServerFull(t, "", true)
	req := httptest.NewRequest("GET", "/view/manga/dummy/manga1", nil)
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)
	// Plugin not loaded — handler returns502 for missing plugin.
	if rec.Code != 502 {
		t.Fatalf("status = %d, want 502; body: %s", rec.Code, rec.Body.String())
	}
}

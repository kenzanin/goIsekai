package httpserver

import (
	"net/http/httptest"
	"strings"
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

// ── Library search (FTS) ─────────────────────────────────────────────────

func TestViewLibrarySearchFiltersNonMatching(t *testing.T) {
	s, db := testServerFullDB(t, "", true)

	seedManga(t, db, "s1|a", "s1", "a", "Solo Leveling")
	seedManga(t, db, "s2|b", "s2", "b", "Berserk")

	req := httptest.NewRequest("GET", "/view/library?q=Solo", nil)
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Solo Leveling") {
		t.Fatalf("expected 'Solo Leveling' in response")
	}
	if strings.Contains(body, "Berserk") {
		t.Fatalf("'Berserk' should be filtered out")
	}
}

func TestViewLibrarySearchHidesStatsRow(t *testing.T) {
	s, db := testServerFullDB(t, "", true)

	seedManga(t, db, "s1|a", "s1", "a", "Solo Leveling")

	req := httptest.NewRequest("GET", "/view/library?q=Solo", nil)
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	// The stats block renders the marker "titles" inside the stat card div.
	// When ?q= is set the stats block is hidden ({{if .Q == ""}}).
	if strings.Contains(body, `text-neutral-400">titles</div>`) {
		t.Fatalf("stats row should be hidden when ?q= is set")
	}
}

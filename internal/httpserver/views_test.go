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

	seedManga(t, db, "s1", "a", "Solo Leveling")
	seedManga(t, db, "s2", "b", "Berserk")

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

func TestViewLibraryPluginIDFilter(t *testing.T) {
	s, db := testServerFullDB(t, "", true)

	seedManga(t, db, "p1", "a", "Solo Leveling")
	seedManga(t, db, "p1", "b", "Solo Side Story")
	seedManga(t, db, "p2", "c", "Berserk")

	tests := []struct {
		name    string
		url     string
		want    []string
		notWant []string
	}{
		{
			name: "filter by plugin",
			url:  "/view/library?pluginID=p1",
			want: []string{
				"Solo Leveling", "Solo Side Story",
				"2 results · 1 page",
				"Source: p1",
				`href="/view/library?pluginID=p1"`,
			},
			notWant: []string{"Berserk"},
		},
		{
			name: "plugin filter combines with q",
			url:  "/view/library?pluginID=p1&q=Solo",
			want: []string{
				"Solo Leveling", "Solo Side Story",
				"2 results · 1 page",
				`name="pluginID" value="p1"`,  // search form keeps the filter
				`href="/view/library?q=Solo"`, // clear chip keeps q
			},
			notWant: []string{"Berserk"},
		},
		{
			name: "q hit outside plugin is scoped away",
			url:  "/view/library?pluginID=p1&q=Berserk",
			want: []string{"0 results · 1 page"},
			notWant: []string{
				`href="/view/manga/p2/c"`, // Berserk card
				`href="/view/manga/p1/`,   // any p1 card
			},
		},
		{
			name:    "other plugin sees only its manga",
			url:     "/view/library?pluginID=p2",
			want:    []string{"Berserk", "1 results · 1 page"},
			notWant: []string{"Solo Leveling", "Solo Side Story"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tc.url, nil)
			rec := httptest.NewRecorder()
			s.Router.ServeHTTP(rec, req)
			if rec.Code != 200 {
				t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
			}
			body := rec.Body.String()
			for _, want := range tc.want {
				if !strings.Contains(body, want) {
					t.Fatalf("expected %q in response for %s", want, tc.url)
				}
			}
			for _, nw := range tc.notWant {
				if strings.Contains(body, nw) {
					t.Fatalf("%q should be filtered out for %s", nw, tc.url)
				}
			}
		})
	}
}

func TestViewLibrarySearchHidesStatsRow(t *testing.T) {
	s, db := testServerFullDB(t, "", true)

	seedManga(t, db, "s1", "a", "Solo Leveling")

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

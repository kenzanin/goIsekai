package httpserver

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"goisekai/internal/database"
)

// writeLivePlugin drops a Lua plugin fixture that answers detail/chapter calls
// successfully, standing in for a normal (non-blocked) source like atsumaru.
func writeLivePlugin(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	src := `PLUGIN = { contract_version = 1, name = "Live" }

function search_manga(a)
  return "[{\"id\":\"zdPX2\",\"title\":\"Building a Monster Girl Harem with Forbidden Science\"}]"
end

function get_manga_detail(a)
  local id = a
  if string.sub(id, 1, 1) == '"' then id = string.sub(id, 2, -2) end
  return host.json.encode({ id = id, title = "Building a Monster Girl Harem with Forbidden Science" })
end

function get_chapter_list(a)
  local id = a
  if string.sub(id, 1, 1) == '"' then id = string.sub(id, 2, -2) end
  return host.json.encode({
    { id = id .. ":chapter-1", manga_id = id, title = "Chapter 1", chapter_num = 1 },
    { id = id .. ":chapter-2", manga_id = id, title = "Chapter 2", chapter_num = 2 },
    { id = id .. ":chapter-3", manga_id = id, title = "Chapter 3", chapter_num = 3 },
  })
end

function get_page_list(a) return "[]" end
`
	if err := os.MkdirAll(filepath.Join(dir, "live"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "live", "main.lua"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// TestMangaDetailUncachedLiveFetch reproduces the HAR 502: search returns a
// result the host never persisted, so clicking the card hits
// /view/manga/<plugin>/<id> with no cached copy. The detail view must fetch it
// live instead of answering 502 "failed to load manga details".
func TestMangaDetailUncachedLiveFetch(t *testing.T) {
	s, db := challengeTestServer(t, writeLivePlugin(t))
	if err := db.RegisterPlugin(database.Plugin{ID: "live", Name: "live", IsActive: true}); err != nil {
		t.Fatalf("register plugin: %v", err)
	}

	req := httptest.NewRequest("GET", "/view/manga/live/zdPX2", nil)
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Building a Monster Girl Harem with Forbidden Science") {
		t.Errorf("manga title missing from detail page")
	}
}

// TestMangaDetailAddToLibraryKeepsChapters reproduces the second HAR error:
// open detail (19 chapters) -> add to library -> redirect renders the detail
// view again and every chapter is gone. Chapters are only persisted for
// library manga, and the manga row IS persisted by the first open, so the
// redirect's cache read finds a row with an empty chapter list — the detail
// view must re-fetch live in that case instead of rendering "No chapters".
func TestMangaDetailAddToLibraryKeepsChapters(t *testing.T) {
	s, db := challengeTestServer(t, writeLivePlugin(t))
	// challengeTestServer only mounts view/static routes; the toggle form
	// posts to /action, so register the action routes for this test.
	s.registerActionRoutes()
	if err := db.RegisterPlugin(database.Plugin{ID: "live", Name: "live", IsActive: true}); err != nil {
		t.Fatalf("register plugin: %v", err)
	}

	// Open the detail page: uncached, so the live fetch renders the chapters.
	req := httptest.NewRequest("GET", "/view/manga/live/zdPX2", nil)
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("initial open: status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "view/read/live/zdPX2/zdPX2:chapter-1") {
		t.Fatalf("initial open: chapter link missing, body: %s", rec.Body.String())
	}

	// Add to library: the real form posts to /action/toggle-library and the
	// browser follows the 303 back to the detail view.
	toggle := csrfPost(s, "/action/toggle-library/live/zdPX2", nil)
	trec := httptest.NewRecorder()
	s.Router.ServeHTTP(trec, toggle)
	if trec.Code != http.StatusSeeOther {
		t.Fatalf("toggle: status = %d, want 303; body: %s", trec.Code, trec.Body.String())
	}

	// Follow the redirect exactly like the browser: re-request the detail view.
	req2 := httptest.NewRequest("GET", "/view/manga/live/zdPX2", nil)
	rec2 := httptest.NewRecorder()
	s.Router.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("after toggle: status = %d, want 200; body: %s", rec2.Code, rec2.Body.String())
	}
	body := rec2.Body.String()
	if strings.Contains(body, "No chapters") {
		t.Errorf("chapter list vanished after adding to library (marker \\\"No chapters\\\" present)")
	}
	for _, id := range []string{"zdPX2:chapter-1", "zdPX2:chapter-2", "zdPX2:chapter-3"} {
		if !strings.Contains(body, "view/read/live/zdPX2/"+id) {
			t.Errorf("chapter %s missing after adding to library", id)
		}
	}

	// And the chapters must now be persisted: the manga is in the library, so
	// the next open serves from cache with no plugin round-trip.
	rowID, _ := db.ResolveMangaIntID("live", "zdPX2")
	n, err := db.CountChaptersForManga(rowID)
	if err != nil {
		t.Fatalf("count chapters: %v", err)
	}
	if n != 3 {
		t.Errorf("persisted chapters = %d, want 3", n)
	}
}

// TestMangaDetailShowsPluginLogo covers the plugin badge image on the detail
// page. A wasm plugin's plugin.json need not declare a logo and its VM
// metadata is runtime-only (PluginMetas is empty until the VM is instantiated),
// so the detail view must resolve the icon from the same DB-first + on-disk
// logo.png fallback every other view uses. It used to read PluginMetas
// directly, which left the badge with no image for exactly these plugins.
func TestMangaDetailShowsPluginLogo(t *testing.T) {
	dir := writeLivePlugin(t)
	s, db := challengeTestServer(t, dir)
	if err := db.RegisterPlugin(database.Plugin{
		ID:       "live",
		Name:     "live",
		Version:  "1",
		WasmPath: filepath.Join(dir, "live", "main.lua"),
		IsActive: true,
	}); err != nil {
		t.Fatalf("register plugin: %v", err)
	}

	req := httptest.NewRequest("GET", "/view/manga/live/zdPX2", nil)
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `<img src="/plugin-static/live/logo.png"`) {
		t.Errorf("plugin logo missing from detail page; body: %s", rec.Body.String())
	}
}

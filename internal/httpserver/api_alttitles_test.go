package httpserver

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"goisekai/internal/bridge"
	"goisekai/internal/database"
	"goisekai/internal/hostnet"
	"goisekai/internal/pluginmanager"
	"goisekai/internal/templates"
)

// testServerFullDB is like testServerFull but also returns the underlying DB
// handle so tests can seed rows directly.
func testServerFullDB(t *testing.T, apiKey string, registerViews bool) (*Server, *database.DB) {
	t.Helper()
	db, err := database.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	proxy := hostnet.NewProxy()
	pmgr := pluginmanager.NewManager(proxy, t.TempDir())
	svc := bridge.NewAppService(db, pmgr, proxy, "", t.TempDir())
	r := chi.NewRouter()
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))

	s := &Server{
		Router:  r,
		logger:  logger,
		service: svc,
		apiKey:  apiKey,
	}
	if registerViews {
		engine, engErr := templates.New(false)
		if engErr != nil {
			t.Fatalf("new engine: %v", engErr)
		}
		s.engine = engine
	}

	s.Router.Route("/api", func(sub chi.Router) {
		sub.Use(s.requireAPIKey)
		s.registerAPIRoutes(sub)
		s.registerReaderRoutes(sub)
		s.registerWSRoutes(sub)
		s.registerSandboxRoutes(sub)
	})
	if registerViews {
		s.registerStaticRoutes()
		s.registerViewRoutes()
		s.registerActionRoutes()
	}
	return s, db
}

// seedManga inserts a manga row with in_library=true and syncs FTS.
func seedManga(t *testing.T, db *database.DB, id, pluginID, sourceID, title string) {
	t.Helper()
	if err := db.UpsertManga(database.Manga{
		ID:            id,
		PluginID:      pluginID,
		SourceMangaID: sourceID,
		Title:         title,
		InLibrary:     true,
	}); err != nil {
		t.Fatalf("upsert manga %s: %v", id, err)
	}
	if err := db.SyncFTS(id); err != nil {
		t.Fatalf("sync fts %s: %v", id, err)
	}
}

// ── GET /api/alt-title-servers ──────────────────────────────────────────────

func TestAltTitleServersReturnsEmptyJSONArray(t *testing.T) {
	s, _ := testServerFullDB(t, "", false)

	req := httptest.NewRequest("GET", "/api/alt-title-servers", nil)
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var arr []map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &arr); err != nil {
		t.Fatalf("body is not valid JSON array: %v\nbody: %s", err, rec.Body.String())
	}
	if len(arr) != 0 {
		t.Fatalf("expected empty array, got %d elements", len(arr))
	}
}

// ── POST /api/manga/../alt-titles ───────────────────────────────────────────

func TestFetchAltTitlesMissingServer(t *testing.T) {
	s, _ := testServerFullDB(t, "", false)

	body := bytes.NewBufferString(`{}`)
	req := httptest.NewRequest("POST", "/api/manga/p1/m1/alt-titles", body)
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestFetchAltTitlesUnknownServer(t *testing.T) {
	s, _ := testServerFullDB(t, "", false)

	body := bytes.NewBufferString(`{"server":"no-such-server"}`)
	req := httptest.NewRequest("POST", "/api/manga/p1/m1/alt-titles", body)
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

// ── PUT title with unknown title ────────────────────────────────────────────

func TestSetTitleUnknownTitle(t *testing.T) {
	s, db := testServerFullDB(t, "", false)

	// Seed a manga with one alt title.
	seedManga(t, db, "p1|m1", "p1", "m1", "Main Title")
	db.AddAltTitles("p1|m1", []string{"Known Alt"}, "src")

	body := bytes.NewBufferString(`{"title":"Totally Unknown"}`)
	req := httptest.NewRequest("PUT", "/api/manga/p1/m1/title", body)
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body: %s", rec.Code, rec.Body.String())
	}
}

// ── DELETE with title equal to main title ───────────────────────────────────

func TestRemoveAltTitleEqualsMainTitle(t *testing.T) {
	s, db := testServerFullDB(t, "", false)

	seedManga(t, db, "p1|m1", "p1", "m1", "Main Title")

	body := bytes.NewBufferString(`{"title":"Main Title"}`)
	req := httptest.NewRequest("DELETE", "/api/manga/p1/m1/alt-titles", body)
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "cannot remove the current main title") {
		t.Fatalf("expected 'cannot remove the current main title', got: %s", rec.Body.String())
	}
}

// ── GET /api/library/search without q ──────────────────────────────────────

func TestLibrarySearchMissingQuery(t *testing.T) {
	s, _ := testServerFullDB(t, "", false)

	req := httptest.NewRequest("GET", "/api/library/search", nil)
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

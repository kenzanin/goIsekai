package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
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

// coverActionsFixture wires a Server whose service talks to a temp DB with
// one library manga already cached under (plugin "t", source id "s1").
func coverActionsFixture(t *testing.T) (*Server, *bridge.AppService) {
	t.Helper()

	db, err := database.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if _, err := db.UpsertManga(database.Manga{
		PluginID: "t", SourceMangaID: "s1", Title: "Fixture", InLibrary: true,
	}); err != nil {
		t.Fatalf("upsert manga: %v", err)
	}

	proxy := hostnet.NewProxy()
	pmgr := pluginmanager.NewManager(proxy, t.TempDir())
	svc := bridge.NewAppService(db, pmgr, proxy, "", t.TempDir(), nil)

	engine, err := templates.New(os.DirFS("../templates"), false)
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}

	s := &Server{
		logger:    slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)),
		service:   svc,
		engine:    engine,
		csrfToken: mintTestCSRF(t),
	}
	return s, svc
}

// coverActionRequest builds a POST whose chi route context carries the
// pluginID/mangaID URL params, the way the live wildcard routes would.
func coverActionRequest(path, pluginID, mangaID, body string) *http.Request {
	req := httptest.NewRequest("POST", path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("pluginID", pluginID)
	rctx.URLParams.Add("mangaID", mangaID)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

// TestSetCoverRejectsEmptyURL pins the 502 contract: a set-cover failure is
// the upstream source's fault, not the client's, so it maps to
// StatusBadGateway — except a malformed body, which stays 400.
func TestSetCoverRejectsEmptyURL(t *testing.T) {
	s, _ := coverActionsFixture(t)

	req := coverActionRequest("/action/set-cover/t/s1", "t", "s1", "url=")
	rec := httptest.NewRecorder()
	s.handleSetCover(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("empty url: got %d, want 502", rec.Code)
	}
}

// TestToggleCoverDimReturnsJSON pins the in-place dim contract: the handler
// answers with {"dim":n} JSON (never a redirect — a redirect re-renders the
// detail page and re-invokes the plugin's GetMangaDetail), and the state
// actually alternates 1 -> 0 in the DB.
func TestToggleCoverDimReturnsJSON(t *testing.T) {
	s, _ := coverActionsFixture(t)

	req := coverActionRequest("/action/toggle-cover-dim/t/s1", "t", "s1", "")
	rec := httptest.NewRecorder()
	s.handleToggleCoverDim(rec, req)

	if rec.Code != 200 {
		t.Fatalf("first toggle: got %d, want 200", rec.Code)
	}
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("body not JSON: %v (%q)", err, rec.Body.String())
	}
	if out["dim"] != float64(1) {
		t.Fatalf("first toggle dim = %v, want 1", out["dim"])
	}

	rec2 := httptest.NewRecorder()
	s.handleToggleCoverDim(rec2, coverActionRequest("/action/toggle-cover-dim/t/s1", "t", "s1", ""))
	if rec2.Code != 200 {
		t.Fatalf("second toggle: got %d, want 200", rec2.Code)
	}
	var out2 map[string]any
	if err := json.Unmarshal(rec2.Body.Bytes(), &out2); err != nil {
		t.Fatalf("second body not JSON: %v", err)
	}
	if out2["dim"] != float64(0) {
		t.Fatalf("second toggle dim = %v, want 0 (must alternate)", out2["dim"])
	}
}

// TestToggleCoverDimUnknownMangaIs400 pins the error path: an unknown
// plugin/manga pair cannot resolve to a row, so the handler answers 400.
func TestToggleCoverDimUnknownMangaIs400(t *testing.T) {
	s, _ := coverActionsFixture(t)

	req := coverActionRequest("/action/toggle-cover-dim/t/missing", "t", "missing", "")
	rec := httptest.NewRecorder()
	s.handleToggleCoverDim(rec, req)

	if rec.Code != 400 {
		t.Fatalf("unknown manga: got %d, want 400", rec.Code)
	}
}

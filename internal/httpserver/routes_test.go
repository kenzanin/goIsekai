package httpserver

import (
	"bytes"
	"log/slog"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"goisekai/internal/bridge"
	"goisekai/internal/database"
	"goisekai/internal/hostnet"
	"goisekai/internal/pluginmanager"
	"goisekai/internal/templates"
)

func TestRenderPageNavToken(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	proxy := hostnet.NewProxy()
	pmgr := pluginmanager.NewManager(proxy, t.TempDir())
	svc := bridge.NewAppService(db, pmgr, proxy, "", t.TempDir(), nil)

	engine, engErr := templates.New(os.DirFS("../templates"), false)
	if engErr != nil {
		t.Fatalf("new engine: %v", engErr)
	}

	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	s := &Server{
		Router:  nil,
		logger:  logger,
		service: svc,
		engine:  engine,
	}

	req := httptest.NewRequest("GET", "/view/library", nil)
	req.Header.Set("X-Partial", "true")
	rec := httptest.NewRecorder()

	s.renderPage(rec, req, "views/library", "library", nil)

	got := rec.Header().Get("X-Active-Nav")
	if got != "library" {
		t.Errorf("X-Active-Nav = %q, want %q", got, "library")
	}

	req2 := httptest.NewRequest("GET", "/view/library", nil)
	rec2 := httptest.NewRecorder()
	s.renderPage(rec2, req2, "views/library", "library", nil)

	if rec2.Header().Get("X-Active-Nav") != "" {
		t.Error("full page should not carry X-Active-Nav")
	}
}

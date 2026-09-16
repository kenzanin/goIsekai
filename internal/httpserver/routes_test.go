package httpserver

import (
	"bytes"
	"log/slog"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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

	// The nav only ships with a full page render, and `active` picks the
	// highlighted link. An unknown token (detail pages, the reader) must
	// highlight nothing rather than leaving the previous page's tab lit.
	req := httptest.NewRequest("GET", "/view/library", nil)
	rec := httptest.NewRecorder()
	s.renderPage(rec, req, "views/library", "library", nil)

	if body := rec.Body.String(); !strings.Contains(body, `data-nav="library"`) {
		t.Fatal("full page render has no nav bar")
	} else if n := strings.Count(body, "border-indigo-400"); n != 1 {
		t.Errorf("library page highlights %d nav link(s), want exactly 1", n)
	}

	req2 := httptest.NewRequest("GET", "/view/library", nil)
	rec2 := httptest.NewRecorder()
	s.renderPage(rec2, req2, "views/library", "", nil)

	if n := strings.Count(rec2.Body.String(), "border-indigo-400"); n != 0 {
		t.Errorf("empty active token highlighted %d nav link(s), want 0", n)
	}
}

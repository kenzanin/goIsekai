package httpserver

import (
	"bytes"
	"log/slog"
	"net/http/httptest"
	"os"
	"testing"

	"goisekai/internal/bridge"
	"goisekai/internal/database"
	"goisekai/internal/hostnet"
	"goisekai/internal/pluginmanager"
	"goisekai/internal/templates"
)

// TestRenderCoverModalHeaders pins the two headers the in-place modal swap
// relies on: the body is always an HTML fragment, and X-Toast is present only
// when there is a toast to show. Without the first the JS swap receives a
// string it cannot parse as markup; without the conditional second, the
// fetch-covers response would surface an empty toast bubble.
func TestRenderCoverModalHeaders(t *testing.T) {
	db, err := database.Open(t.TempDir() + "/test.db")
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

	s := &Server{
		logger:    slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)),
		service:   svc,
		engine:    engine,
		csrfToken: mintTestCSRF(t),
	}

	const pluginID, mangaID = "p1", "s1"
	if _, upsertErr := db.UpsertManga(database.Manga{
		PluginID:      pluginID,
		SourceMangaID: mangaID,
		Title:         "Test Manga",
		CoverURL:      "https://example.test/cover.jpg",
	}); upsertErr != nil {
		t.Fatalf("upsert manga: %v", upsertErr)
	}

	tests := []struct {
		name      string
		toast     string
		wantToast bool
	}{
		{name: "no toast", toast: "", wantToast: false},
		{name: "with toast", toast: "Cover updated", wantToast: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			if renderErr := s.renderCoverModal(rec, pluginID, mangaID, tt.toast); renderErr != nil {
				t.Fatalf("renderCoverModal: %v", renderErr)
			}

			if got, want := rec.Header().Get("Content-Type"), "text/html; charset=utf-8"; got != want {
				t.Errorf("Content-Type = %q, want %q", got, want)
			}
			gotToast := rec.Header().Get("X-Toast")
			if tt.wantToast && gotToast != tt.toast {
				t.Errorf("X-Toast = %q, want %q", gotToast, tt.toast)
			}
			if !tt.wantToast && gotToast != "" {
				t.Errorf("X-Toast = %q, want absent", gotToast)
			}
			if rec.Body.Len() == 0 {
				t.Error("empty body; the swap target would be cleared")
			}
		})
	}
}

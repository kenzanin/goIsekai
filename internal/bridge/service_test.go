package bridge

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"goisekai/internal/database"
	"goisekai/internal/hostnet"
	"goisekai/pkg/types"
)

// newTestService builds an AppService backed by a throwaway SQLite file and a
// real hostnet proxy. The plugin manager is left nil: every path exercised
// below either delegates to the db/proxy or to the persist helper directly.
func newTestService(t *testing.T) *AppService {
	t.Helper()
	db, err := database.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return NewAppService(db, nil, hostnet.NewProxy(), "", "")
}

func TestGetMangaDetailsPersists(t *testing.T) {
	s := newTestService(t)

	manga := types.Manga{
		ID:       "source-42",
		Title:    "Cheat Chef in the Far Land",
		CoverURL: "http://example.com/cover.jpg",
		Status:   "Ongoing",
	}
	chapters := []types.Chapter{
		{ID: "ch-1", MangaID: "source-42", Title: "Chapter 1", ChapterNum: 1, VolumeNum: 0, ReleasedAt: time.Unix(1700000000, 0)},
		{ID: "ch-2", MangaID: "source-42", Title: "Chapter 2", ChapterNum: 2, VolumeNum: 0, ReleasedAt: time.Unix(1700086400, 0)},
	}

	// Exercise the persistence mapping directly (no plugin manager needed).
	if err := s.persistMangaDetails("plugin-a", manga, chapters); err != nil {
		t.Fatalf("persistMangaDetails: %v", err)
	}

	lib, err := s.ListLibrary()
	if err != nil {
		t.Fatalf("ListLibrary: %v", err)
	}
	if len(lib) != 0 {
		t.Fatalf("expected empty library for freshly upserted (in_library=0) manga, got %d", len(lib))
	}

	// ToggleLibraryItem takes source ids; the bridge reconstructs the row id.
	if err := s.ToggleLibraryItem("plugin-a", "source-42"); err != nil {
		t.Fatalf("ToggleLibraryItem: %v", err)
	}
	lib, err = s.ListLibrary()
	if err != nil {
		t.Fatalf("ListLibrary after toggle: %v", err)
	}
	if len(lib) != 1 {
		t.Fatalf("expected 1 library item after toggle, got %d", len(lib))
	}
	got := lib[0]
	rowID := "plugin-a|source-42"
	if got.ID != rowID {
		t.Errorf("manga id = %q, want %q", got.ID, rowID)
	}
	if got.SourceMangaID != manga.ID {
		t.Errorf("source_manga_id = %q, want %q", got.SourceMangaID, manga.ID)
	}
	if got.PluginID != "plugin-a" {
		t.Errorf("plugin_id = %q, want %q", got.PluginID, "plugin-a")
	}
	if got.Title != manga.Title {
		t.Errorf("title = %q, want %q", got.Title, manga.Title)
	}
	if !got.InLibrary {
		t.Error("expected in_library = true after toggle")
	}
}

func TestSetChapterProgress(t *testing.T) {
	s := newTestService(t)
	manga := types.Manga{ID: "source-9", Title: "Progress Manga"}
	chapters := []types.Chapter{{ID: "ch-7", MangaID: "source-9", Title: "Chapter 7", ChapterNum: 7}}

	if err := s.persistMangaDetails("plugin-b", manga, chapters); err != nil {
		t.Fatalf("persistMangaDetails: %v", err)
	}

	if err := s.SetChapterProgress("plugin-b", "source-9", "ch-7", 3); err != nil {
		t.Fatalf("SetChapterProgress: %v", err)
	}
}

func TestToggleLibraryRoundTrip(t *testing.T) {
	s := newTestService(t)
	manga := types.Manga{ID: "source-100", Title: "Toggle Manga"}
	if err := s.persistMangaDetails("plugin-c", manga, nil); err != nil {
		t.Fatalf("persistMangaDetails: %v", err)
	}

	lib, _ := s.ListLibrary()
	if len(lib) != 0 {
		t.Fatalf("expected empty library initially, got %d", len(lib))
	}
	if err := s.ToggleLibraryItem("plugin-c", "source-100"); err != nil {
		t.Fatalf("toggle on: %v", err)
	}
	lib, _ = s.ListLibrary()
	if len(lib) != 1 {
		t.Fatalf("expected 1 item after toggle on, got %d", len(lib))
	}
	if err := s.ToggleLibraryItem("plugin-c", "source-100"); err != nil {
		t.Fatalf("toggle off: %v", err)
	}
	lib, _ = s.ListLibrary()
	if len(lib) != 0 {
		t.Fatalf("expected empty library after toggle off, got %d", len(lib))
	}
}

func TestGetImageCaches(t *testing.T) {
	var hits atomic.Int32
	payload := []byte("\x89PNG\r\n\x1a\n-static-bytes-")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(payload)
	}))
	defer srv.Close()

	s := newTestService(t)

	first, err := s.GetImage("plugin-x", srv.URL+"/img.png", nil, "", "")
	if err != nil {
		t.Fatalf("GetImage first call: %v", err)
	}
	second, err := s.GetImage("plugin-x", srv.URL+"/img.png", nil, "", "")
	if err != nil {
		t.Fatalf("GetImage second call: %v", err)
	}
	if string(first) != string(payload) {
		t.Errorf("first bytes = %q, want %q", first, payload)
	}
	if string(second) != string(payload) {
		t.Errorf("second bytes = %q, want %q", second, payload)
	}
	if got := hits.Load(); got != 1 {
		t.Errorf("expected exactly 1 network hit (2nd served from cache), got %d", got)
	}
}

func TestGetImageNonSuccessNotCached(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	s := newTestService(t)
	if _, err := s.GetImage("plugin-x", srv.URL+"/missing.png", nil, "", ""); err == nil {
		t.Fatal("expected error for non-2xx response")
	}
	// A fresh call for the failed URL must hit the network again (error not cached).
	if _, err := s.GetImage("plugin-x", srv.URL+"/missing.png", nil, "", ""); err == nil {
		t.Fatal("expected error on second call too")
	}
	// Each GetImage retries up to 3 attempts (at-home burst rate-limit
	// workaround), so two calls observe 6 upstream hits.
	if got := hits.Load(); got != 6 {
		t.Errorf("expected 6 network hits (2 calls x 3 attempts, errors not cached), got %d", got)
	}
}

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

// TestGetMangaDetailsNilManagerFallback tests that GetMangaDetails falls back
// to cached DB data when the plugin manager is nil (simulating plugin-unreachable).
func TestGetMangaDetailsNilManagerFallback(t *testing.T) {
	t.Parallel()
	db, err := database.Open(filepath.Join(t.TempDir(), "fallback.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer func() { _ = db.Close() }()

	// Pre-populate DB with a cached manga + chapters.
	mangaID, err := db.UpsertManga(database.Manga{
		PluginID: "offline-plugin", SourceMangaID: "src-99",
		Title: "Offline Manga", CoverURL: "http://example.com/offline.jpg",
		Description: "Offline description", Status: "Ongoing",
	})
	if err != nil {
		t.Fatalf("upsert manga: %v", err)
	}
	chapters := []database.Chapter{
		{MangaID: mangaID, SourceChapterID: "ch-1", Title: "Ch1", ChapterNum: 1},
		{MangaID: mangaID, SourceChapterID: "ch-2", Title: "Ch2", ChapterNum: 2},
	}
	for _, c := range chapters {
		chapterID, err := db.UpsertChapter(c)
		if err != nil {
			t.Fatalf("upsert chapter %s: %v", c.SourceChapterID, err)
		}
		_ = chapterID
	}

	// Build a service with nil manager — simulates plugin-unreachable state.
	s := NewAppService(db, nil, hostnet.NewProxy(), "", "", nil)

	manga, mangaChapters, err := s.GetMangaDetails("offline-plugin", "src-99")
	if err != nil {
		t.Fatalf("GetMangaDetails (nil mgr): %v", err)
	}
	if manga.Title != "Offline Manga" {
		t.Errorf("manga title = %q, want %q", manga.Title, "Offline Manga")
	}
	if manga.CoverURL != "http://example.com/offline.jpg" {
		t.Errorf("manga cover = %q, want %q", manga.CoverURL, "http://example.com/offline.jpg")
	}
	if manga.Description != "Offline description" {
		t.Errorf("manga desc = %q, want %q", manga.Description, "Offline description")
	}
	if len(mangaChapters) != 2 {
		t.Fatalf("expected 2 cached chapters, got %d", len(mangaChapters))
	}
	// Chapters should be newest-first.
	if mangaChapters[0].ChapterNum != 2 {
		t.Errorf("chapter[0].ChapterNum = %v, want 2", mangaChapters[0].ChapterNum)
	}
	if mangaChapters[1].ChapterNum != 1 {
		t.Errorf("chapter[1].ChapterNum = %v, want 1", mangaChapters[1].ChapterNum)
	}
}

// TestGetMangaDetailsCacheMiss tests that GetMangaDetails returns an error
// when the plugin manager is nil AND no cached data exists.
func TestGetMangaDetailsCacheMiss(t *testing.T) {
	t.Parallel()
	db, err := database.Open(filepath.Join(t.TempDir(), "miss.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer func() { _ = db.Close() }()

	s := NewAppService(db, nil, hostnet.NewProxy(), "", "", nil)

	_, _, err = s.GetMangaDetails("ghost-plugin", "ghost-src")
	if err == nil {
		t.Fatal("expected error for missing cached manga, got nil")
	}
}

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
	return NewAppService(db, nil, hostnet.NewProxy(), "", "", nil)
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
	if got.PluginID != "plugin-a" {
		t.Errorf("plugin_id = %q, want %q", got.PluginID, "plugin-a")
	}
	if got.SourceMangaID != manga.ID {
		t.Errorf("source_manga_id = %q, want %q", got.SourceMangaID, manga.ID)
	}
	if got.ID == 0 {
		t.Error("expected non-zero integer ID")
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

// TestPersistMangaDetailsSkipsNonLibraryChapters: chapters are mirrored only for
// library manga. A detail-view cache row must not drag a chapter list into the
// database, while the caller still returns the live list it fetched.
func TestPersistMangaDetailsSkipsNonLibraryChapters(t *testing.T) {
	s := newTestService(t)
	manga := types.Manga{ID: "source-77", Title: "Detail Cache Only"}
	chapters := []types.Chapter{{ID: "c1", MangaID: "source-77", Title: "Ch 1", ChapterNum: 1}}

	if err := s.persistMangaDetails("plugin-z", manga, chapters); err != nil {
		t.Fatalf("persistMangaDetails: %v", err)
	}
	mangaIntID, err := s.db.ResolveMangaIntID("plugin-z", "source-77")
	if err != nil {
		t.Fatalf("resolve manga: %v", err)
	}
	if got, err := s.db.ListChaptersCached(mangaIntID); err != nil {
		t.Fatalf("ListChaptersCached: %v", err)
	} else if len(got) != 0 {
		t.Fatalf("non-library manga persisted %d chapters, want none", len(got))
	}

	// Once it is in the library the same call does persist them.
	if err := s.ToggleLibraryItem("plugin-z", "source-77"); err != nil {
		t.Fatalf("ToggleLibraryItem: %v", err)
	}
	if err := s.persistMangaDetails("plugin-z", manga, chapters); err != nil {
		t.Fatalf("persistMangaDetails (in library): %v", err)
	}
	if got, err := s.db.ListChaptersCached(mangaIntID); err != nil {
		t.Fatalf("ListChaptersCached: %v", err)
	} else if len(got) != 1 {
		t.Fatalf("in-library manga persisted %d chapters, want 1", len(got))
	}
}

// TestReadingFlowPrunesPageCacheAndKeepsProgress walks the reader's sequence:
// cache a page list, mark a chapter read, read a page of another, then confirm a
// prune drops the finished chapter's cached pages (so the reader re-fetches
// them) while the progress it reports survives.
func TestReadingFlowPrunesPageCacheAndKeepsProgress(t *testing.T) {
	s := newTestService(t)

	if _, err := s.db.UpsertManga(database.Manga{
		PluginID: "plugin-r", SourceMangaID: "source-5", Title: "Reader", InLibrary: true,
	}); err != nil {
		t.Fatalf("upsert manga: %v", err)
	}
	if err := s.persistMangaDetails("plugin-r",
		types.Manga{ID: "source-5", Title: "Reader"},
		[]types.Chapter{
			{ID: "ch-1", MangaID: "source-5", Title: "Chapter 1", ChapterNum: 1},
			{ID: "ch-2", MangaID: "source-5", Title: "Chapter 2", ChapterNum: 2},
		}); err != nil {
		t.Fatalf("persistMangaDetails: %v", err)
	}

	ch1, err := s.db.ResolveChapterIntID("plugin-r", "source-5", "ch-1")
	if err != nil || ch1 == 0 {
		t.Fatalf("resolve ch-1: id=%d err=%v", ch1, err)
	}
	ch2, err := s.db.ResolveChapterIntID("plugin-r", "source-5", "ch-2")
	if err != nil || ch2 == 0 {
		t.Fatalf("resolve ch-2: id=%d err=%v", ch2, err)
	}

	pages := []byte(`[{"URL":"http://example.com/1.png","Index":1}]`)
	if err := s.db.SaveChapterPages(ch1, pages); err != nil {
		t.Fatalf("cache ch-1 pages: %v", err)
	}
	if err := s.MarkChapterRead("plugin-r", "source-5", "ch-1"); err != nil {
		t.Fatalf("MarkChapterRead: %v", err)
	}
	if err := s.db.SaveChapterPages(ch2, pages); err != nil {
		t.Fatalf("cache ch-2 pages: %v", err)
	}
	if err := s.SetChapterProgress("plugin-r", "source-5", "ch-2", 5); err != nil {
		t.Fatalf("SetChapterProgress: %v", err)
	}

	assertProgress := func(when string) {
		t.Helper()
		progress, err := s.GetChapterProgresses("plugin-r", "source-5")
		if err != nil {
			t.Fatalf("GetChapterProgresses %s: %v", when, err)
		}
		if !progress["ch-1"].Done {
			t.Errorf("%s: ch-1 not marked read", when)
		}
		if got := progress["ch-2"].LastPageRead; got != 5 {
			t.Errorf("%s: ch-2 last page = %d, want 5", when, got)
		}
	}
	assertProgress("after reading")

	if _, err := s.db.PruneOrphans(); err != nil {
		t.Fatalf("PruneOrphans: %v", err)
	}

	// The finished chapter's cached list is gone, so the reader re-fetches it.
	if cached, err := s.db.GetChapterPages(ch1); err != nil {
		t.Fatalf("GetChapterPages ch-1: %v", err)
	} else if cached != nil {
		t.Errorf("finished chapter kept its cached page list")
	}
	// The chapter still being read keeps its cache.
	if cached, err := s.db.GetChapterPages(ch2); err != nil {
		t.Fatalf("GetChapterPages ch-2: %v", err)
	} else if cached == nil {
		t.Errorf("in-progress chapter lost its cached page list")
	}
	// Pruning storage must not lose the reading state the UI renders.
	assertProgress("after prune")
	mangaIntID, err := s.db.ResolveMangaIntID("plugin-r", "source-5")
	if err != nil {
		t.Fatalf("resolve manga: %v", err)
	}
	if inLibrary, err := s.db.IsInLibrary(mangaIntID); err != nil {
		t.Fatalf("IsInLibrary: %v", err)
	} else if !inLibrary {
		t.Error("library manga was pruned")
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

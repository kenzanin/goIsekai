package bridge

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"goisekai/internal/database"
	"goisekai/internal/hostnet"
	"goisekai/internal/pluginmanager"
)

// A cached EMPTY page list must read as a miss, not as an answer.
//
// The reader falls back to a live fetch only when the cached result is nil. A
// plugin that failed to list its pages once would therefore have pinned the
// chapter to "no pages" for good - and the reader could not recover even
// after the plugin was fixed.
func TestGetPageListCachedEmptyCountsAsMiss(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "emptycache.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer func() { _ = db.Close() }()

	mangaIntID, err := db.UpsertManga(database.Manga{
		PluginID: "p", SourceMangaID: "m1", Title: "M",
	})
	if err != nil {
		t.Fatalf("upsert manga: %v", err)
	}
	chapterIntID, err := db.UpsertChapter(database.Chapter{
		MangaID: mangaIntID, SourceChapterID: "ch-1", Title: "Ch1", ChapterNum: 1,
	})
	if err != nil {
		t.Fatalf("upsert chapter: %v", err)
	}
	if err := db.SaveChapterPages(chapterIntID, []byte(`[]`)); err != nil {
		t.Fatalf("save empty pages: %v", err)
	}

	s := NewAppService(db, nil, hostnet.NewProxy(), "", "", nil)
	t.Cleanup(s.Shutdown)

	pages, err := s.GetPageListCached("p", "ch-1")
	if err != nil {
		t.Fatalf("GetPageListCached: %v", err)
	}
	if pages != nil {
		t.Fatalf("cached empty list returned %d pages, want nil (a miss)", len(pages))
	}
}

// A good cached list is still served as-is.
func TestGetPageListCachedKeepsGoodList(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "goodcache.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer func() { _ = db.Close() }()

	mangaIntID, err := db.UpsertManga(database.Manga{
		PluginID: "p", SourceMangaID: "m1", Title: "M",
	})
	if err != nil {
		t.Fatalf("upsert manga: %v", err)
	}
	chapterIntID, err := db.UpsertChapter(database.Chapter{
		MangaID: mangaIntID, SourceChapterID: "ch-1", Title: "Ch1", ChapterNum: 1,
	})
	if err != nil {
		t.Fatalf("upsert chapter: %v", err)
	}
	good := []byte(`[{"index":0,"url":"https://example.com/1.png"}]`)
	if err := db.SaveChapterPages(chapterIntID, good); err != nil {
		t.Fatalf("save pages: %v", err)
	}

	s := NewAppService(db, nil, hostnet.NewProxy(), "", "", nil)
	t.Cleanup(s.Shutdown)

	pages, err := s.GetPageListCached("p", "ch-1")
	if err != nil {
		t.Fatalf("GetPageListCached: %v", err)
	}
	if len(pages) != 1 {
		t.Fatalf("got %d pages, want 1", len(pages))
	}
	if pages[0].URL != "https://example.com/1.png" {
		t.Errorf("page url = %q, want the cached url", pages[0].URL)
	}
}

// An empty answer from the plugin must not overwrite a good cache.
//
// The plugin is the only writer of chapter_pages, so an empty list stored here
// is what a transient upstream failure would leave behind.
func TestGetPageListEmptyAnswerKeepsGoodCache(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "nocache.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer func() { _ = db.Close() }()

	mangaIntID, err := db.UpsertManga(database.Manga{
		PluginID: "luaemptypages", SourceMangaID: "m1", Title: "M",
	})
	if err != nil {
		t.Fatalf("upsert manga: %v", err)
	}
	// The plugin must be active: an inactive plugin takes the offline path and
	// serves the cache, which would not exercise the live answer at all.
	if err := db.RegisterPlugin(database.Plugin{ID: "luaemptypages", Name: "LuaEmptyPages", IsActive: true}); err != nil {
		t.Fatalf("register plugin: %v", err)
	}
	chapterIntID, err := db.UpsertChapter(database.Chapter{
		MangaID: mangaIntID, SourceChapterID: "ch-1", Title: "Ch1", ChapterNum: 1,
	})
	if err != nil {
		t.Fatalf("upsert chapter: %v", err)
	}
	good := []byte(`[{"index":0,"url":"https://example.com/1.png"}]`)
	if err := db.SaveChapterPages(chapterIntID, good); err != nil {
		t.Fatalf("save pages: %v", err)
	}

	pluginsDir := t.TempDir()
	dst := filepath.Join(pluginsDir, "luaemptypages")
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatalf("mkdir fixture: %v", err)
	}
	src, err := os.ReadFile(filepath.Join("..", "pluginmanager", "testdata", "luaemptypages", "main.lua"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dst, "main.lua"), src, 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	mgr := pluginmanager.NewManager(hostnet.NewProxy(), pluginsDir)
	mgr.SetDB(db, time.Hour)
	mgr.SetChapterCacheTTL(168 * time.Hour)
	if err := mgr.Discover(); err != nil {
		t.Fatalf("discover: %v", err)
	}
	defer func() { _ = mgr.Close() }()

	s := NewAppService(db, mgr, hostnet.NewProxy(), "", "", nil)
	t.Cleanup(s.Shutdown)

	pages, err := s.GetPageList("luaemptypages", "ch-1")
	if err != nil {
		t.Fatalf("GetPageList: %v", err)
	}
	if len(pages) != 0 {
		t.Fatalf("got %d pages from the empty-pages fixture, want 0", len(pages))
	}

	stored, err := db.GetChapterPages(chapterIntID)
	if err != nil {
		t.Fatalf("read stored pages: %v", err)
	}
	if string(stored) != string(good) {
		t.Errorf("cache clobbered by an empty answer: got %q, want %q", stored, good)
	}
}

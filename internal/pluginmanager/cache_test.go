package pluginmanager

import (
	"path/filepath"
	"testing"
	"time"

	"goisekai/internal/database"
	"goisekai/internal/hostnet"
)

// TestChapterCacheTTL verifies that GetChapterList uses chapterCacheTTL
// while GetMangaDetail uses cacheTTL, and that backdated cache entries
// are still served within their TTL window.
func TestChapterCacheTTL(t *testing.T) {
	// Create a temp plugins dir with the luatest fixture.
	pluginsDir := t.TempDir()
	dst := filepath.Join(pluginsDir, "luatest")
	if err := copyDir("testdata/luatest", dst); err != nil {
		t.Fatalf("copy fixture: %v", err)
	}

	// Create a temp DB.
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := database.Open(dbPath)
	if err != nil {
		t.Fatalf("database.Open: %v", err)
	}
	defer func() { _ = db.Close() }()

	mgr := NewManager(hostnet.NewProxy(), pluginsDir)
	mgr.SetDB(db, 24*time.Hour)
	mgr.SetChapterCacheTTL(168 * time.Hour)
	if err := mgr.Discover(); err != nil {
		t.Fatalf("Discover: %v", err)
	}
	defer func() { _ = mgr.Close() }()

	// First GetChapterList call should invoke the plugin.
	chapters1, err := mgr.GetChapterList("luatest", "m1")
	if err != nil {
		t.Fatalf("GetChapterList (first): %v", err)
	}
	if len(chapters1) != 1 {
		t.Fatalf("expected 1 chapter, got %d", len(chapters1))
	}

	// Second call within TTL should serve from cache (no plugin re-invoke).
	chapters2, err := mgr.GetChapterList("luatest", "m1")
	if err != nil {
		t.Fatalf("GetChapterList (cached): %v", err)
	}
	if len(chapters2) != 1 {
		t.Fatalf("expected 1 cached chapter, got %d", len(chapters2))
	}

	// Insert a backdated cache entry AFTER the second call so it
	// overwrites the fresh cache entry from the second GetChapterList.
	// Simulate an entry cached 25h ago with 168h TTL (143h remain).
	_, err = db.Exec(
		`INSERT OR REPLACE INTO plugin_cache (id, plugin_id, manga_id, function_name, response, cached_at, expires_at)
		 VALUES (?, 'luatest', 'm1', 'get_chapter_list', '[{"id":"m1/c1","manga_id":"m1","title":"Ch 1","chapter_num":1.0}]', datetime('now', '-25 hours'), datetime('now', '+143 hours'))`,
		"luatest\x00m1\x00get_chapter_list",
	)
	if err != nil {
		t.Fatalf("insert backdated cache: %v", err)
	}

	// The backdated entry (143h remaining) should still be served
	// because it hasn't expired. This proves chapterCacheTTL=168h
	// is used, not the default 24h cacheTTL.
	chapters3, err := mgr.GetChapterList("luatest", "m1")
	if err != nil {
		t.Fatalf("GetChapterList (backdated cache): %v", err)
	}
	if len(chapters3) != 1 {
		t.Fatalf("expected 1 chapter from backdated cache, got %d", len(chapters3))
	}
}

// TestDetailCacheTTL verifies that GetMangaDetail uses cacheTTL
// and does not use chapterCacheTTL.
func TestDetailCacheTTL(t *testing.T) {
	pluginsDir := t.TempDir()
	dst := filepath.Join(pluginsDir, "luatest")
	if err := copyDir("testdata/luatest", dst); err != nil {
		t.Fatalf("copy fixture: %v", err)
	}

	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := database.Open(dbPath)
	if err != nil {
		t.Fatalf("database.Open: %v", err)
	}
	defer func() { _ = db.Close() }()

	mgr := NewManager(hostnet.NewProxy(), pluginsDir)
	mgr.SetDB(db, 24*time.Hour)
	mgr.SetChapterCacheTTL(168 * time.Hour)
	if err := mgr.Discover(); err != nil {
		t.Fatalf("Discover: %v", err)
	}
	defer func() { _ = mgr.Close() }()

	// First GetMangaDetail call should invoke the plugin.
	detail1, err := mgr.GetMangaDetail("luatest", "m1")
	if err != nil {
		t.Fatalf("GetMangaDetail (first): %v", err)
	}
	if detail1.Title != "Detail m1" {
		t.Fatalf("unexpected detail title: %s", detail1.Title)
	}

	// Second call within TTL should serve from cache.
	detail2, err := mgr.GetMangaDetail("luatest", "m1")
	if err != nil {
		t.Fatalf("GetMangaDetail (cached): %v", err)
	}
	if detail2.Title != "Detail m1" {
		t.Fatalf("unexpected cached detail title: %s", detail2.Title)
	}
}

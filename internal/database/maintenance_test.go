package database

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBackupToCreatesFileAndPrunes(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	d, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = d.Close() }()

	dir := filepath.Join(t.TempDir(), "backups")
	for i := range 7 {
		if _, err := d.BackupTo(dir, 5); err != nil {
			t.Fatalf("backup %d: %v", i, err)
		}
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 5 {
		t.Fatalf("want 5 backups, got %d", len(entries))
	}
}

func TestPruneOrphans(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	d, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = d.Close() }()

	// Insert manga + chapter + history, then delete manga to create orphans
	res, err := d.db.Exec(`INSERT INTO mangas (plugin_id, source_manga_id, title, in_library) VALUES ('p1','s1','T',0)`)
	if err != nil {
		t.Fatal(err)
	}
	mangaID, err := res.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	cres, err := d.db.Exec(`INSERT INTO chapters (manga_id, source_chapter_id, title, chapter_num) VALUES (?, 'sc1','Ch1',1)`, mangaID)
	if err != nil {
		t.Fatal(err)
	}
	chapterID, err := cres.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.db.Exec(`INSERT INTO read_history (chapter_id, page_num) VALUES (?, 1)`, chapterID); err != nil {
		t.Fatal(err)
	}
	// Delete manga -> chapters + history orphaned
	if _, err := d.db.Exec(`DELETE FROM mangas WHERE id = ?`, mangaID); err != nil {
		t.Fatal(err)
	}

	summary, err := d.PruneOrphans()
	if err != nil {
		t.Fatal(err)
	}

	var h, c int
	_ = d.db.QueryRow(`SELECT COUNT(*) FROM read_history`).Scan(&h)
	_ = d.db.QueryRow(`SELECT COUNT(*) FROM chapters`).Scan(&c)
	if h != 0 || c != 0 {
		t.Fatalf("orphans remain: history=%d chapters=%d summary=%q", h, c, summary)
	}
}

// TestPruneOrphansPurgePolicy covers the storage rules: chapters of non-library
// manga go away, cached pages of finished chapters go away, and the negatives
// (library chapters, unread chapters' pages) survive.
func TestPruneOrphansPurgePolicy(t *testing.T) {
	d := openTestDB(t)

	count := func(query string, args ...any) int {
		var n int
		if err := d.db.QueryRow(query, args...).Scan(&n); err != nil {
			t.Fatalf("count %q: %v", query, err)
		}
		return n
	}

	libraryID, err := d.UpsertManga(Manga{PluginID: "p1", SourceMangaID: "lib", Title: "Kept", InLibrary: true})
	if err != nil {
		t.Fatalf("upsert library manga: %v", err)
	}
	cacheID, err := d.UpsertManga(Manga{PluginID: "p1", SourceMangaID: "cache", Title: "Detail cache"})
	if err != nil {
		t.Fatalf("upsert cache manga: %v", err)
	}

	keepChapter, err := d.UpsertChapter(Chapter{MangaID: libraryID, SourceChapterID: "keep", Title: "Keep", ChapterNum: 1})
	if err != nil {
		t.Fatalf("upsert unread chapter: %v", err)
	}
	doneChapter, err := d.UpsertChapter(Chapter{MangaID: libraryID, SourceChapterID: "done", Title: "Done", ChapterNum: 2, IsRead: true})
	if err != nil {
		t.Fatalf("upsert finished chapter: %v", err)
	}
	cacheChapter, err := d.UpsertChapter(Chapter{MangaID: cacheID, SourceChapterID: "cc", Title: "Cached", ChapterNum: 1})
	if err != nil {
		t.Fatalf("upsert cache chapter: %v", err)
	}

	for _, id := range []int64{keepChapter, doneChapter, cacheChapter} {
		if err := d.SaveChapterPages(id, []byte(`[]`)); err != nil {
			t.Fatalf("save pages %d: %v", id, err)
		}
		if err := d.RecordRead(id, 1); err != nil {
			t.Fatalf("record read %d: %v", id, err)
		}
	}

	if _, err := d.PruneOrphans(); err != nil {
		t.Fatalf("PruneOrphans: %v", err)
	}

	if got := count(`SELECT COUNT(*) FROM chapter_pages WHERE chapter_id = ?`, keepChapter); got != 1 {
		t.Errorf("unread library chapter lost its page cache (%d rows)", got)
	}
	if got := count(`SELECT COUNT(*) FROM read_history WHERE chapter_id = ?`, keepChapter); got != 1 {
		t.Errorf("library chapter lost its history row (%d rows)", got)
	}
	if got := count(`SELECT COUNT(*) FROM chapter_pages WHERE chapter_id = ?`, doneChapter); got != 0 {
		t.Errorf("finished chapter kept %d cached page rows, want 0", got)
	}
	if got := count(`SELECT COUNT(*) FROM read_history WHERE chapter_id = ?`, doneChapter); got != 1 {
		t.Errorf("finished chapter lost its history row (%d rows)", got)
	}
	if got := count(`SELECT COUNT(*) FROM chapters WHERE manga_id = ?`, cacheID); got != 0 {
		t.Errorf("non-library manga kept %d chapters, want 0", got)
	}
	if got := count(`SELECT COUNT(*) FROM mangas WHERE id = ?`, cacheID); got != 0 {
		t.Errorf("non-library manga row survived, want it pruned")
	}
	if got := count(`SELECT COUNT(*) FROM mangas WHERE id = ?`, libraryID); got != 1 {
		t.Errorf("library manga was pruned")
	}
}

// TestPurgeRemovedPlugin covers the destructive path the "Purge removed"
// maintenance button runs: every row keyed to one plugin disappears while the
// other plugin's rows survive. Two traps are pinned here — alt_covers has no
// ON DELETE CASCADE (unlike alt_titles), so it only goes if the purge deletes
// it explicitly, and library_fts is a virtual table whose columns carry no
// type affinity, so a delete bound with the wrong type matches zero rows and
// silently leaves a stale index behind.
func TestPurgeRemovedPlugin(t *testing.T) {
	d := openTestDB(t)

	count := func(query string, args ...any) int {
		var n int
		if err := d.db.QueryRow(query, args...).Scan(&n); err != nil {
			t.Fatalf("count %q: %v", query, err)
		}
		return n
	}

	for _, id := range []string{"gone", "kept"} {
		if err := d.RegisterPlugin(Plugin{ID: id, Name: id, Version: "1", WasmPath: "/nonexistent/" + id, IsActive: true}); err != nil {
			t.Fatalf("register %s: %v", id, err)
		}
		if _, err := d.db.Exec(`INSERT INTO plugin_verify (plugin_id, verify_url, cookies, user_agent) VALUES (?, 'https://x', 'ck', 'ua')`, id); err != nil {
			t.Fatalf("plugin_verify %s: %v", id, err)
		}
		mid, err := d.UpsertManga(Manga{PluginID: id, SourceMangaID: "s1", Title: "Title " + id, InLibrary: true})
		if err != nil {
			t.Fatalf("upsert %s: %v", id, err)
		}
		if _, err := d.db.Exec(`INSERT INTO alt_titles (manga_row_id, title, source) VALUES (?, ?, 'test')`, mid, "Alt "+id); err != nil {
			t.Fatalf("alt_titles %s: %v", id, err)
		}
		if _, err := d.db.Exec(`INSERT INTO alt_covers (manga_row_id, url, source) VALUES (?, ?, 'test')`, mid, "https://img/"+id); err != nil {
			t.Fatalf("alt_covers %s: %v", id, err)
		}
		cid, err := d.UpsertChapter(Chapter{MangaID: mid, SourceChapterID: "c1", Title: "Ch", ChapterNum: 1})
		if err != nil {
			t.Fatalf("chapter %s: %v", id, err)
		}
		if err := d.RecordRead(cid, 1); err != nil {
			t.Fatalf("record read %s: %v", id, err)
		}
		if _, err := d.db.Exec(`INSERT INTO plugin_cache (id, plugin_id, manga_id, function_name, response, expires_at) VALUES (?, ?, 's1', 'GetMangaDetail', '{}', '2099-01-01')`, "pc-"+id, id); err != nil {
			t.Fatalf("plugin_cache %s: %v", id, err)
		}
	}

	// Setup guard: without rows on both sides every later "is 0" assertion
	// would pass even if the purge did nothing.
	for table, want := range map[string]int{
		"mangas": 2, "chapters": 2, "read_history": 2, "alt_titles": 2,
		"alt_covers": 2, "plugin_cache": 2, "plugin_verify": 2,
		"plugins": 2, "library_fts": 2,
	} {
		if got := count(`SELECT COUNT(*) FROM ` + table); got != want {
			t.Fatalf("setup: %s has %d rows, want %d", table, got, want)
		}
	}

	if _, err := d.PurgeRemovedPlugin("gone"); err != nil {
		t.Fatalf("PurgeRemovedPlugin: %v", err)
	}
	if err := d.Vacuum(); err != nil {
		t.Fatalf("Vacuum: %v", err)
	}

	if got := count(`SELECT COUNT(*) FROM mangas WHERE plugin_id = 'gone'`); got != 0 {
		t.Errorf("gone manga survived: %d", got)
	}
	if got := count(`SELECT COUNT(*) FROM chapters`); got != 1 {
		t.Errorf("chapters = %d, want 1 (gone's chapter cascaded, kept's stayed)", got)
	}
	if got := count(`SELECT COUNT(*) FROM read_history`); got != 1 {
		t.Errorf("read_history = %d, want 1", got)
	}
	if got := count(`SELECT COUNT(*) FROM alt_titles`); got != 1 {
		t.Errorf("alt_titles = %d, want 1", got)
	}
	// alt_covers has no ON DELETE CASCADE — only an explicit delete clears it.
	if got := count(`SELECT COUNT(*) FROM alt_covers`); got != 1 {
		t.Errorf("alt_covers = %d, want 1 (no cascade: purge must delete it)", got)
	}
	if got := count(`SELECT COUNT(*) FROM plugin_cache`); got != 1 {
		t.Errorf("plugin_cache = %d, want 1", got)
	}
	if got := count(`SELECT COUNT(*) FROM plugin_verify`); got != 1 {
		t.Errorf("plugin_verify = %d, want 1", got)
	}
	if got := count(`SELECT COUNT(*) FROM plugins WHERE id = 'gone'`); got != 0 {
		t.Errorf("plugins row for the removed plugin survived")
	}
	// The stale-index trap: gone's FTS document must be gone, kept's must not.
	if got := count(`SELECT COUNT(*) FROM library_fts WHERE plugin_id = 'gone'`); got != 0 {
		t.Errorf("library_fts still indexes the removed plugin: %d rows", got)
	}
	if got := count(`SELECT COUNT(*) FROM library_fts WHERE plugin_id = 'kept'`); got != 1 {
		t.Errorf("library_fts lost the surviving plugin's document: %d rows", got)
	}

	if _, err := d.PurgeRemovedPlugin(""); err == nil {
		t.Error("empty plugin id should be rejected")
	}
}

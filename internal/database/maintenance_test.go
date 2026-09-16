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

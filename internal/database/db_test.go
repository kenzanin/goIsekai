package database

import (
	"path/filepath"
	"testing"
)

func openTestDB(t *testing.T) *DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestMigrationsRun(t *testing.T) {
	db := openTestDB(t)

	rows, err := db.db.Query(`SELECT name FROM sqlite_master WHERE type='table' ORDER BY name`)
	if err != nil {
		t.Fatalf("sqlite_master: %v", err)
	}
	defer rows.Close()

	want := map[string]bool{"mangas": false, "chapters": false, "read_history": false, "plugins": false}
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			t.Fatalf("scan table: %v", err)
		}
		if _, ok := want[n]; ok {
			want[n] = true
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows: %v", err)
	}
	for name, seen := range want {
		if !seen {
			t.Errorf("missing table %q", name)
		}
	}
}

func TestUpsertMangaUniqueConstraint(t *testing.T) {
	db := openTestDB(t)

	m := Manga{ID: "m1", PluginID: "p1", SourceMangaID: "s1", Title: "First", InLibrary: true}
	if err := db.UpsertManga(m); err != nil {
		t.Fatalf("upsert 1: %v", err)
	}
	m.Title = "Second"
	if err := db.UpsertManga(m); err != nil {
		t.Fatalf("upsert 2: %v", err)
	}

	// Different source id for the same plugin must be a distinct row.
	m2 := Manga{ID: "m2", PluginID: "p1", SourceMangaID: "s2", Title: "Other"}
	if err := db.UpsertManga(m2); err != nil {
		t.Fatalf("upsert 3: %v", err)
	}

	var count int
	if err := db.db.QueryRow(`SELECT COUNT(*) FROM mangas`).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 rows, got %d", count)
	}

	// The upsert should have updated the title in place.
	var title string
	if err := db.db.QueryRow(`SELECT title FROM mangas WHERE id = ?`, "m1").Scan(&title); err != nil {
		t.Fatalf("title: %v", err)
	}
	if title != "Second" {
		t.Fatalf("expected title updated to Second, got %q", title)
	}
}

func TestToggleLibrary(t *testing.T) {
	db := openTestDB(t)

	m := Manga{ID: "m1", PluginID: "p1", SourceMangaID: "s1", Title: "X", InLibrary: false}
	if err := db.UpsertManga(m); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	var inLib int
	if err := db.db.QueryRow(`SELECT in_library FROM mangas WHERE id = ?`, "m1").Scan(&inLib); err != nil {
		t.Fatalf("scan initial: %v", err)
	}
	if inLib != 0 {
		t.Fatalf("expected in_library=0, got %d", inLib)
	}

	if err := db.ToggleLibrary("m1"); err != nil {
		t.Fatalf("toggle 1: %v", err)
	}
	if err := db.db.QueryRow(`SELECT in_library FROM mangas WHERE id = ?`, "m1").Scan(&inLib); err != nil {
		t.Fatalf("scan after toggle: %v", err)
	}
	if inLib != 1 {
		t.Fatalf("expected in_library=1 after toggle, got %d", inLib)
	}

	if err := db.ToggleLibrary("m1"); err != nil {
		t.Fatalf("toggle 2: %v", err)
	}
	if err := db.db.QueryRow(`SELECT in_library FROM mangas WHERE id = ?`, "m1").Scan(&inLib); err != nil {
		t.Fatalf("scan after toggle back: %v", err)
	}
	if inLib != 0 {
		t.Fatalf("expected in_library=0 after second toggle, got %d", inLib)
	}
}

func TestRecordReadCascade(t *testing.T) {
	db := openTestDB(t)

	m := Manga{ID: "m1", PluginID: "p1", SourceMangaID: "s1", Title: "X"}
	if err := db.UpsertManga(m); err != nil {
		t.Fatalf("upsert manga: %v", err)
	}
	c := Chapter{ID: "c1", MangaID: "m1", SourceChapterID: "cs1", Title: "Ch1", ChapterNum: 1}
	if err := db.UpsertChapter(c); err != nil {
		t.Fatalf("upsert chapter: %v", err)
	}
	if err := db.RecordRead("c1", 5); err != nil {
		t.Fatalf("record read: %v", err)
	}

	// Verify rows exist before deletion.
	var chapterCount, historyCount int
	if err := db.db.QueryRow(`SELECT COUNT(*) FROM chapters WHERE manga_id = ?`, "m1").Scan(&chapterCount); err != nil {
		t.Fatalf("chapter count: %v", err)
	}
	if chapterCount != 1 {
		t.Fatalf("expected 1 chapter, got %d", chapterCount)
	}
	if err := db.db.QueryRow(`SELECT COUNT(*) FROM read_history`).Scan(&historyCount); err != nil {
		t.Fatalf("history count: %v", err)
	}
	if historyCount != 1 {
		t.Fatalf("expected 1 history row, got %d", historyCount)
	}

	// Deleting the manga must cascade-delete its chapters and read_history rows.
	if _, err := db.db.Exec(`DELETE FROM mangas WHERE id = ?`, "m1"); err != nil {
		t.Fatalf("delete manga: %v", err)
	}

	if err := db.db.QueryRow(`SELECT COUNT(*) FROM chapters`).Scan(&chapterCount); err != nil {
		t.Fatalf("chapter count after: %v", err)
	}
	if chapterCount != 0 {
		t.Fatalf("expected 0 chapters after cascade, got %d", chapterCount)
	}
	if err := db.db.QueryRow(`SELECT COUNT(*) FROM read_history`).Scan(&historyCount); err != nil {
		t.Fatalf("history count after: %v", err)
	}
	if historyCount != 0 {
		t.Fatalf("expected 0 history rows after cascade, got %d", historyCount)
	}
}

func TestUpsertChapterPreservesProgress(t *testing.T) {
	db := openTestDB(t)

	m := Manga{ID: "m1", PluginID: "p1", SourceMangaID: "s1", Title: "X"}
	if err := db.UpsertManga(m); err != nil {
		t.Fatalf("upsert manga: %v", err)
	}

	c := Chapter{ID: "c1", MangaID: "m1", SourceChapterID: "cs1", Title: "Ch1", ChapterNum: 1, IsRead: true, LastPageRead: 42, DownloadStatus: DownloadDownloaded}
	if err := db.UpsertChapter(c); err != nil {
		t.Fatalf("upsert 1: %v", err)
	}

	// Refresh with different metadata; progress must survive.
	c.Title = "Ch1 Updated"
	if err := db.UpsertChapter(c); err != nil {
		t.Fatalf("upsert 2: %v", err)
	}

	var title string
	var isRead int
	var lastPage int
	var status string
	if err := db.db.QueryRow(`SELECT title, is_read, last_page_read, download_status FROM chapters WHERE id = ?`, "c1").
		Scan(&title, &isRead, &lastPage, &status); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if title != "Ch1 Updated" {
		t.Fatalf("title not updated: %q", title)
	}
	if isRead != 1 || lastPage != 42 || status != DownloadDownloaded {
		t.Fatalf("progress not preserved: read=%d page=%d status=%q", isRead, lastPage, status)
	}
}

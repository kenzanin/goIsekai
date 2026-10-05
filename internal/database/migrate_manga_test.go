package database

import (
	"path/filepath"
	"testing"
)

func TestRepointManga(t *testing.T) {
	t.Parallel()
	db, err := Open(filepath.Join(t.TempDir(), "repoint.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	id, err := db.UpsertManga(Manga{PluginID: "old", SourceMangaID: "src1", Title: "Old Title", CoverURL: "http://old/cover.jpg", Description: "old desc", Status: "Ongoing", InLibrary: true})
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	var rowID string
	_ = db.db.QueryRow(`SELECT id FROM mangas WHERE plugin_id = ? AND source_manga_id = ?`, "old", "src1").Scan(&rowID)
	if _, err := db.AddAltTitles(rowID, []string{"Alt One"}, "mangadex"); err != nil {
		t.Fatalf("add alt: %v", err)
	}
	if err := db.AddCategory(rowID, "Action"); err != nil {
		_, _ = db.Exec(`INSERT OR IGNORE INTO manga_categories (manga_row_id, category, source) VALUES (?, ?, ?)`, rowID, "Action", "mangadex")
	}

	if err := db.RepointManga(id, "new", "src2", "New Title", "http://new/cover.jpg", "new desc", "Completed"); err != nil {
		t.Fatalf("repoint: %v", err)
	}
	// Row id unchanged: lookup old pair should fail, new pair should return same id.
	if m, _ := db.GetMangaCached("old", "src1"); m.ID != 0 {
		t.Fatalf("old pair still resolves: %v", m)
	}
	m2, err := db.GetMangaCached("new", "src2")
	if err != nil || m2.ID != id {
		t.Fatalf("new pair id = %d, want %d, err %v", m2.ID, id, err)
	}
	if m2.Title != "New Title" {
		t.Fatalf("title = %q, want %q", m2.Title, "New Title")
	}
	// Alt titles still attached to same row id.
	alts, _ := db.ListAltTitles(rowID)
	if len(alts) == 0 {
		t.Fatalf("alt titles lost after repoint")
	}
	// Custom title guard: set custom_title=1, repoint with different title, title must not change.
	_, _ = db.Exec(`UPDATE mangas SET custom_title = 1, title = ? WHERE id = ?`, "Custom Title", id)
	if err := db.RepointManga(id, "new2", "src3", "Overwrite Title", "http://x", "x", "Ongoing"); err != nil {
		t.Fatalf("repoint custom: %v", err)
	}
	var title string
	_ = db.db.QueryRow(`SELECT title FROM mangas WHERE id = ?`, id).Scan(&title)
	if title != "Custom Title" {
		t.Fatalf("custom title overwritten: %q", title)
	}
}

func TestDeleteChaptersNotIn(t *testing.T) {
	t.Parallel()
	db, err := Open(filepath.Join(t.TempDir(), "prune.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	mangaID, _ := db.UpsertManga(Manga{PluginID: "p", SourceMangaID: "m1", Title: "T", InLibrary: true})
	chIDs := []string{"ch1", "ch2", "ch3"}
	for _, sid := range chIDs {
		if _, err := db.UpsertChapter(Chapter{MangaID: mangaID, SourceChapterID: sid, Title: sid, ChapterNum: float64(len(sid))}); err != nil {
			t.Fatalf("upsert ch %s: %v", sid, err)
		}
	}
	// Add another manga's chapter to ensure untouched.
	otherID, _ := db.UpsertManga(Manga{PluginID: "p", SourceMangaID: "m2", Title: "Other", InLibrary: true})
	_, _ = db.UpsertChapter(Chapter{MangaID: otherID, SourceChapterID: "other-ch", Title: "other", ChapterNum: 1})

	// Attach chapter_pages and read_history to ch1 to verify cascade.
	var ch1ID int64
	_ = db.db.QueryRow(`SELECT id FROM chapters WHERE manga_id = ? AND source_chapter_id = ?`, mangaID, "ch1").Scan(&ch1ID)
	_, _ = db.Exec(`INSERT OR REPLACE INTO chapter_pages (chapter_id, pages) VALUES (?, ?)`, ch1ID, `[]`)
	_, _ = db.Exec(`INSERT INTO read_history (chapter_id, page_num) VALUES (?, ?)`, ch1ID, 1)

	n, err := db.DeleteChaptersNotIn(mangaID, []string{"ch2", "ch3"})
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if n != 1 {
		t.Fatalf("removed = %d, want 1", n)
	}
	if c, _ := db.CountChaptersForManga(mangaID); c != 2 {
		t.Fatalf("remaining chapters = %d, want 2", c)
	}
	// Cascaded rows gone.
	var cnt int
	_ = db.db.QueryRow(`SELECT COUNT(*) FROM chapter_pages WHERE chapter_id = ?`, ch1ID).Scan(&cnt)
	if cnt != 0 {
		t.Fatalf("chapter_pages not cascaded")
	}
	_ = db.db.QueryRow(`SELECT COUNT(*) FROM read_history WHERE chapter_id = ?`, ch1ID).Scan(&cnt)
	if cnt != 0 {
		t.Fatalf("read_history not cascaded")
	}
	// Other manga untouched.
	if c, _ := db.CountChaptersForManga(otherID); c != 1 {
		t.Fatalf("other manga chapters = %d, want 1", c)
	}
}

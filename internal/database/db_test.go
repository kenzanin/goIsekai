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
	t.Cleanup(func() {
		_ = db.Close()
	})
	return db
}

func TestMigrationsRun(t *testing.T) {
	db := openTestDB(t)

	rows, err := db.db.Query(`SELECT name FROM sqlite_master WHERE type='table' ORDER BY name`)
	if err != nil {
		t.Fatalf("sqlite_master: %v", err)
	}
	defer func() {
		_ = rows.Close()
	}()

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

// TestPersistenceAcrossRestart verifies that library bookmarks and chapter
// progress survive a close/reopen of the same SQLite file (criterion 7.3).
func TestPersistenceAcrossRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "restart.db")

	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	m := Manga{ID: "m1", PluginID: "p1", SourceMangaID: "s1", Title: "Persist", InLibrary: true}
	if err := db.UpsertManga(m); err != nil {
		t.Fatalf("upsert manga: %v", err)
	}
	c := Chapter{ID: "c1", MangaID: "m1", SourceChapterID: "cs1", Title: "Ch1", ChapterNum: 1}
	if err := db.UpsertChapter(c); err != nil {
		t.Fatalf("upsert chapter: %v", err)
	}
	if err := db.SetChapterProgress("c1", 7); err != nil {
		t.Fatalf("set progress: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	// "Restart": reopen the same file and read back the persisted state.
	db2, err := Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer func() {
		_ = db2.Close()
	}()

	lib, err := db2.ListLibrary()
	if err != nil {
		t.Fatalf("ListLibrary: %v", err)
	}
	if len(lib) != 1 || lib[0].Title != "Persist" || !lib[0].InLibrary {
		t.Fatalf("library not persisted: %+v", lib)
	}

	var isRead, lastPage int
	if err := db2.db.QueryRow(`SELECT is_read, last_page_read FROM chapters WHERE id = ?`, "c1").Scan(&isRead, &lastPage); err != nil {
		t.Fatalf("scan chapter: %v", err)
	}
	// SetChapterProgress records the page but does NOT mark read.
	if isRead != 0 || lastPage != 7 {
		t.Fatalf("progress not persisted: read=%d page=%d", isRead, lastPage)
	}
}

// TestMarkChapterRead covers the single-chapter mark-as-read path.
func TestMarkChapterRead(t *testing.T) {
	db := openTestDB(t)

	if err := db.UpsertManga(Manga{ID: "m1", PluginID: "p1", SourceMangaID: "s1", Title: "R"}); err != nil {
		t.Fatalf("upsert manga: %v", err)
	}
	if err := db.UpsertChapter(Chapter{ID: "c1", MangaID: "m1", SourceChapterID: "cs1", Title: "Ch1", ChapterNum: 1}); err != nil {
		t.Fatalf("upsert chapter: %v", err)
	}
	if err := db.MarkChapterRead("c1"); err != nil {
		t.Fatalf("MarkChapterRead: %v", err)
	}
	var isRead, lastPage int
	if err := db.db.QueryRow(`SELECT is_read, last_page_read FROM chapters WHERE id = ?`, "c1").Scan(&isRead, &lastPage); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if isRead != 1 {
		t.Fatalf("is_read=%d, want 1", isRead)
	}
	if lastPage != 0 {
		t.Fatalf("last_page_read=%d, want 0 (mark-read must not touch page)", lastPage)
	}
}

func chapterProgressBySource(t *testing.T, db *DB, mangaRowID string) map[string]ChapterProgress {
	t.Helper()
	rows, err := db.GetChapterProgressForManga(mangaRowID)
	if err != nil {
		t.Fatalf("GetChapterProgressForManga: %v", err)
	}
	m := make(map[string]ChapterProgress, len(rows))
	for _, r := range rows {
		m[r.SourceChapterID] = r
	}
	return m
}

// TestSetChaptersBulkRead covers the action dropdown's bulk toggles: an explicit
// selection, the "up to" boundary (highest selected chapter), and whole-manga
// toggles — all without disturbing per-chapter page progress.
func TestSetChaptersBulkRead(t *testing.T) {
	db := openTestDB(t)

	if err := db.UpsertManga(Manga{ID: "m1", PluginID: "p1", SourceMangaID: "s1", Title: "B"}); err != nil {
		t.Fatalf("upsert manga: %v", err)
	}
	for _, c := range []Chapter{
		{ID: "c1", MangaID: "m1", SourceChapterID: "cs1", Title: "A", ChapterNum: 1},
		{ID: "c2", MangaID: "m1", SourceChapterID: "cs2", Title: "B", ChapterNum: 2},
		{ID: "c3", MangaID: "m1", SourceChapterID: "cs3", Title: "C", ChapterNum: 3},
		{ID: "c4", MangaID: "m1", SourceChapterID: "cs4", Title: "D", ChapterNum: 4},
	} {
		if err := db.UpsertChapter(c); err != nil {
			t.Fatalf("upsert chapter %s: %v", c.ID, err)
		}
	}
	// Page progress must survive every one of these toggles.
	if err := db.SetChapterProgress("c1", 5); err != nil {
		t.Fatalf("set progress: %v", err)
	}

	// Explicit selection: only the listed chapters flip.
	if err := db.SetChaptersRead("m1", []string{"cs1", "cs3"}, true); err != nil {
		t.Fatalf("SetChaptersRead: %v", err)
	}
	p := chapterProgressBySource(t, db, "m1")
	if !p["cs1"].IsRead || !p["cs3"].IsRead {
		t.Fatalf("selected chapters should be read: %+v", p)
	}
	if p["cs2"].IsRead || p["cs4"].IsRead {
		t.Fatalf("unselected chapters must be untouched: %+v", p)
	}
	if p["cs1"].LastPageRead != 5 {
		t.Fatalf("page progress must survive mark-read: %+v", p["cs1"])
	}

	// Unmark restores the flag but keeps page progress.
	if err := db.SetChaptersRead("m1", []string{"cs1"}, false); err != nil {
		t.Fatalf("SetChaptersRead unread: %v", err)
	}
	p = chapterProgressBySource(t, db, "m1")
	if p["cs1"].IsRead || p["cs1"].LastPageRead != 5 {
		t.Fatalf("cs1 should be unread with progress intact: %+v", p["cs1"])
	}

	// "Up to" boundary is the highest chapter_num of the selection: from cs3
	// that means cs1..cs3 become read, cs4 stays as-is.
	if err := db.SetChaptersRead("m1", []string{"cs1", "cs3"}, false); err != nil {
		t.Fatalf("reset flags: %v", err)
	}
	if err := db.SetChaptersUpTo("m1", []string{"cs3"}, true); err != nil {
		t.Fatalf("SetChaptersUpTo: %v", err)
	}
	p = chapterProgressBySource(t, db, "m1")
	for _, src := range []string{"cs1", "cs2", "cs3"} {
		if !p[src].IsRead {
			t.Fatalf("%s should be read by up-to cs3: %+v", src, p[src])
		}
	}
	if p["cs4"].IsRead {
		t.Fatalf("cs4 is past the boundary: %+v", p["cs4"])
	}

	// A multi-selection uses its maximum as the boundary (cs2,cs4 -> 4).
	if err := db.SetChaptersUpTo("m1", []string{"cs2", "cs4"}, false); err != nil {
		t.Fatalf("SetChaptersUpTo clear: %v", err)
	}
	p = chapterProgressBySource(t, db, "m1")
	for src, c := range p {
		if c.IsRead {
			t.Fatalf("%s should be cleared by up-to cs4: %+v", src, c)
		}
	}
	if err := db.SetChaptersUpTo("m1", []string{"cs9"}, true); err == nil {
		t.Fatal("unknown chapter should error")
	}

	// Whole-manga toggle.
	if err := db.SetMangaChaptersRead("m1", true); err != nil {
		t.Fatalf("SetMangaChaptersRead: %v", err)
	}
	p = chapterProgressBySource(t, db, "m1")
	for src, c := range p {
		if !c.IsRead {
			t.Fatalf("%s should be read: %+v", src, c)
		}
	}
	if err := db.SetMangaChaptersRead("m1", false); err != nil {
		t.Fatalf("SetMangaChaptersRead unread: %v", err)
	}
	for src, c := range chapterProgressBySource(t, db, "m1") {
		if c.IsRead {
			t.Fatalf("%s should be unread: %+v", src, c)
		}
	}
}

// TestChapterProgressForManga covers total-pages persistence and the
// per-manga progress read used by the detail-page badges and Continue button.
func TestChapterProgressForManga(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "progress.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := db.UpsertManga(Manga{ID: "m1", PluginID: "p1", SourceMangaID: "s1", Title: "P"}); err != nil {
		t.Fatalf("upsert manga: %v", err)
	}
	for _, c := range []Chapter{
		{ID: "c1", MangaID: "m1", SourceChapterID: "cs1", Title: "Ch1", ChapterNum: 1},
		{ID: "c2", MangaID: "m1", SourceChapterID: "cs2", Title: "Ch2", ChapterNum: 2},
	} {
		if err := db.UpsertChapter(c); err != nil {
			t.Fatalf("upsert chapter %s: %v", c.ID, err)
		}
	}
	if err := db.SetChapterTotalPages("c1", 18); err != nil {
		t.Fatalf("set total pages: %v", err)
	}
	if err := db.SetChapterProgress("c1", 5); err != nil {
		t.Fatalf("set progress: %v", err)
	}

	rows, err := db.GetChapterProgressForManga("m1")
	if err != nil {
		t.Fatalf("GetChapterProgressForManga: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("want 2 rows, got %d", len(rows))
	}
	bySource := map[string]ChapterProgress{}
	for _, r := range rows {
		bySource[r.SourceChapterID] = r
	}
	p1 := bySource["cs1"]
	// Progress alone (5/18) must NOT mark the chapter read or done.
	if p1.IsRead || p1.Done || p1.LastPageRead != 5 || p1.TotalPages != 18 {
		t.Fatalf("cs1 progress wrong: %+v", p1)
	}
	if p2 := bySource["cs2"]; p2.LastPageRead != 0 || p2.TotalPages != 0 || p2.IsRead || p2.Done {
		t.Fatalf("cs2 should be untouched: %+v", p2)
	}
}

// TestChapterDoneDerivation covers the "read" strikethrough semantics: a
// chapter is Done when manually marked read OR fully read (last >= total > 0),
// and ResetChapterProgress clears both signals.
func TestChapterDoneDerivation(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "done.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := db.UpsertManga(Manga{ID: "m1", PluginID: "p1", SourceMangaID: "s1", Title: "D"}); err != nil {
		t.Fatalf("upsert manga: %v", err)
	}
	for _, c := range []Chapter{
		{ID: "c1", MangaID: "m1", SourceChapterID: "cs1", Title: "A", ChapterNum: 1},
		{ID: "c2", MangaID: "m1", SourceChapterID: "cs2", Title: "B", ChapterNum: 2},
		{ID: "c3", MangaID: "m1", SourceChapterID: "cs3", Title: "C", ChapterNum: 3},
	} {
		if err := db.UpsertChapter(c); err != nil {
			t.Fatalf("upsert chapter %s: %v", c.ID, err)
		}
	}

	get := func() map[string]ChapterProgress {
		rows, err := db.GetChapterProgressForManga("m1")
		if err != nil {
			t.Fatalf("GetChapterProgressForManga: %v", err)
		}
		m := map[string]ChapterProgress{}
		for _, r := range rows {
			m[r.SourceChapterID] = r
		}
		return m
	}

	// Fully read: last == total == 10 => Done, IsRead false.
	if err := db.SetChapterTotalPages("c1", 10); err != nil {
		t.Fatalf("set total pages: %v", err)
	}
	if err := db.SetChapterProgress("c1", 10); err != nil {
		t.Fatalf("set progress: %v", err)
	}
	p := get()["cs1"]
	if p.IsRead || !p.Done {
		t.Fatalf("fully-read chapter should be Done but not IsRead: %+v", p)
	}

	// Manually marked read with no page read => Done via IsRead.
	if err := db.MarkChapterRead("c2"); err != nil {
		t.Fatalf("mark read: %v", err)
	}
	p = get()["cs2"]
	if !p.IsRead || !p.Done {
		t.Fatalf("manually-read chapter should be IsRead and Done: %+v", p)
	}

	// Reset clears both the full-read and the manual-read chapter.
	for _, id := range []string{"c1", "c2"} {
		if err := db.ResetChapterProgress(id); err != nil {
			t.Fatalf("reset chapter progress %s: %v", id, err)
		}
	}
	for _, src := range []string{"cs1", "cs2", "cs3"} {
		p := get()[src]
		if p.IsRead || p.Done || p.LastPageRead != 0 {
			t.Fatalf("reset should clear %s: %+v", src, p)
		}
	}
}

// TestNewBadgeLifecycle covers the library card's [New] badge: CountChapters
// grows when a new chapter is stored, MarkMangaNew stamps the badge, and
// ClearMangaNew (detail opened) clears it.
func TestNewBadgeLifecycle(t *testing.T) {
	db := openTestDB(t)

	if err := db.UpsertManga(Manga{ID: "m1", PluginID: "p1", SourceMangaID: "s1", Title: "B", InLibrary: true}); err != nil {
		t.Fatalf("upsert manga: %v", err)
	}
	if err := db.UpsertChapter(Chapter{ID: "c1", MangaID: "m1", SourceChapterID: "cs1", Title: "Ch1", ChapterNum: 1}); err != nil {
		t.Fatalf("upsert chapter: %v", err)
	}

	count := func() int {
		t.Helper()
		n, err := db.CountChaptersForManga("m1")
		if err != nil {
			t.Fatalf("CountChaptersForManga: %v", err)
		}
		return n
	}
	stats := func() LibraryMangaStats {
		t.Helper()
		rows, err := db.ListLibraryWithProgress()
		if err != nil || len(rows) != 1 {
			t.Fatalf("ListLibraryWithProgress: err=%v rows=%d", err, len(rows))
		}
		return rows[0]
	}

	// Freshly synced manga: no badge.
	if count() != 1 {
		t.Fatalf("want 1 chapter, got %d", count())
	}
	if s := stats(); s.HasNew {
		t.Fatalf("badge must be off before any sync finds new chapters")
	}

	// Sync finds a new chapter: count grows, badge goes on.
	if err := db.UpsertChapter(Chapter{ID: "c2", MangaID: "m1", SourceChapterID: "cs2", Title: "Ch2", ChapterNum: 2}); err != nil {
		t.Fatalf("upsert new chapter: %v", err)
	}
	if err := db.MarkMangaNew("m1"); err != nil {
		t.Fatalf("MarkMangaNew: %v", err)
	}
	if count() != 2 {
		t.Fatalf("want 2 chapters, got %d", count())
	}
	if s := stats(); !s.HasNew || s.TotalChapters != 2 {
		t.Fatalf("badge must be on after sync found a new chapter: %+v", s)
	}

	// Opening the manga clears the badge.
	if err := db.ClearMangaNew("p1", "s1"); err != nil {
		t.Fatalf("ClearMangaNew: %v", err)
	}
	if s := stats(); s.HasNew {
		t.Fatalf("badge must be off after the manga was opened: %+v", s)
	}
}

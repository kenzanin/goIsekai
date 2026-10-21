package database

import (
	"testing"
)

// A plugin that re-issues a chapter under a new id (it now answers
// "slug:chapter-42" where the row still holds "chapter-42") leaves the old row
// behind forever. The prune removes it, but only with complete proof.
func TestPruneSupersededChaptersRemovesReissuedRow(t *testing.T) {
	db := openTestDB(t)
	mangaID, err := db.UpsertManga(Manga{PluginID: "p", SourceMangaID: "m1", Title: "M"})
	if err != nil {
		t.Fatalf("UpsertManga: %v", err)
	}
	for _, id := range []string{"chapter-42", "chapter-99"} {
		if _, err := db.UpsertChapter(Chapter{
			MangaID:         mangaID,
			SourceChapterID: id,
			Title:           id,
			ChapterNum:      42,
		}); err != nil {
			t.Fatalf("UpsertChapter %s: %v", id, err)
		}
	}

	n, err := db.PruneSupersededChapters(mangaID, []Chapter{{SourceChapterID: "m1:chapter-42", ChapterNum: 42}})
	if err != nil {
		t.Fatalf("PruneSupersededChapters: %v", err)
	}
	if n != 1 {
		t.Fatalf("removed %d rows, want 1", n)
	}

	ids := chapterIDs(t, db, mangaID)
	if ids["chapter-42"] {
		t.Error("the re-issued row should be gone")
	}
	if !ids["chapter-99"] {
		t.Error("a row without a replacement in the answer must stay")
	}
}

// The proof must be complete: same chapter number, no read progress.
func TestPruneSupersededChaptersKeepsUnprovenRows(t *testing.T) {
	db := openTestDB(t)
	mangaID, err := db.UpsertManga(Manga{PluginID: "p", SourceMangaID: "m1", Title: "M"})
	if err != nil {
		t.Fatalf("UpsertManga: %v", err)
	}
	read, err := db.UpsertChapter(Chapter{
		MangaID:         mangaID,
		SourceChapterID: "chapter-7",
		Title:           "ch 7",
		ChapterNum:      7,
	})
	if err != nil {
		t.Fatalf("UpsertChapter: %v", err)
	}
	if err := db.MarkChapterRead(read); err != nil {
		t.Fatalf("MarkChapterRead: %v", err)
	}
	other, err := db.UpsertChapter(Chapter{
		MangaID:         mangaID,
		SourceChapterID: "chapter-8",
		Title:           "ch 8",
		ChapterNum:      8,
	})
	if err != nil {
		t.Fatalf("UpsertChapter: %v", err)
	}
	if err := db.MarkChapterRead(other); err != nil {
		t.Fatalf("MarkChapterRead: %v", err)
	}
	// Same tail, different chapter number: no proof which one replaced what.
	if _, err := db.UpsertChapter(Chapter{
		MangaID:         mangaID,
		SourceChapterID: "chapter-9",
		Title:           "ch 9",
		ChapterNum:      9,
	}); err != nil {
		t.Fatalf("UpsertChapter: %v", err)
	}

	n, err := db.PruneSupersededChapters(mangaID, []Chapter{
		{SourceChapterID: "m1:chapter-7", ChapterNum: 7},  // read already
		{SourceChapterID: "m1:chapter-9", ChapterNum: 99}, // number mismatch
	})
	if err != nil {
		t.Fatalf("PruneSupersededChapters: %v", err)
	}
	if n != 0 {
		t.Fatalf("removed %d rows, want 0", n)
	}
	ids := chapterIDs(t, db, mangaID)
	for _, id := range []string{"chapter-7", "chapter-8", "chapter-9"} {
		if !ids[id] {
			t.Errorf("%s must stay", id)
		}
	}
}

// An empty answer proves nothing: a plugin that failed to list its chapters
// must not wipe the cache.
func TestPruneSupersededChaptersEmptyAnswerRemovesNothing(t *testing.T) {
	db := openTestDB(t)
	mangaID, err := db.UpsertManga(Manga{PluginID: "p", SourceMangaID: "m1", Title: "M"})
	if err != nil {
		t.Fatalf("UpsertManga: %v", err)
	}
	if _, err := db.UpsertChapter(Chapter{
		MangaID:         mangaID,
		SourceChapterID: "chapter-42",
		Title:           "ch 42",
		ChapterNum:      42,
	}); err != nil {
		t.Fatalf("UpsertChapter: %v", err)
	}

	n, err := db.PruneSupersededChapters(mangaID, nil)
	if err != nil {
		t.Fatalf("PruneSupersededChapters: %v", err)
	}
	if n != 0 {
		t.Fatalf("removed %d rows, want 0", n)
	}
	if !chapterIDs(t, db, mangaID)["chapter-42"] {
		t.Error("chapter-42 must stay")
	}
}

// A row the plugin still sends is never a candidate, and a short tail must
// not match an unrelated chapter.
func TestPruneSupersededChaptersIgnoresLiveRows(t *testing.T) {
	db := openTestDB(t)
	mangaID, err := db.UpsertManga(Manga{PluginID: "p", SourceMangaID: "m1", Title: "M"})
	if err != nil {
		t.Fatalf("UpsertManga: %v", err)
	}
	if _, err := db.UpsertChapter(Chapter{
		MangaID:         mangaID,
		SourceChapterID: "m1:chapter-42",
		Title:           "ch 42",
		ChapterNum:      42,
	}); err != nil {
		t.Fatalf("UpsertChapter: %v", err)
	}

	n, err := db.PruneSupersededChapters(mangaID, []Chapter{{SourceChapterID: "m1:chapter-42", ChapterNum: 42}})
	if err != nil {
		t.Fatalf("PruneSupersededChapters: %v", err)
	}
	if n != 0 {
		t.Fatalf("removed %d rows, want 0", n)
	}
	if !chapterIDs(t, db, mangaID)["m1:chapter-42"] {
		t.Error("a row the plugin still sends must stay")
	}
}

// chapterIDs reports which source ids a manga still has.
func chapterIDs(t *testing.T, db *DB, mangaID int64) map[string]bool {
	t.Helper()
	rows, err := db.ListChaptersCached(mangaID)
	if err != nil {
		t.Fatalf("ListChaptersCached: %v", err)
	}
	out := make(map[string]bool, len(rows))
	for _, c := range rows {
		out[c.SourceChapterID] = true
	}
	return out
}

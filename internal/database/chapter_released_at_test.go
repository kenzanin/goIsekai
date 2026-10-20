package database

import (
	"testing"
	"time"
)

// A chapter's release date is the site's own upload date, reported by the
// plugin. It must survive the round trip through the database, and it must
// stay separate from fetched_at (when WE last synced the row).
func TestUpsertChapterKeepsReleaseDate(t *testing.T) {
	db := openTestDB(t)

	mangaID, err := db.UpsertManga(Manga{PluginID: "p", SourceMangaID: "m1", Title: "M"})
	if err != nil {
		t.Fatalf("UpsertManga: %v", err)
	}

	released := time.Date(2024, 9, 7, 17, 4, 15, 0, time.UTC)
	if _, err := db.UpsertChapter(Chapter{
		MangaID:         mangaID,
		SourceChapterID: "ch1",
		Title:           "Chapter 1",
		ChapterNum:      1,
		ReleasedAt:      released,
	}); err != nil {
		t.Fatalf("UpsertChapter: %v", err)
	}

	chapters, err := db.ListChaptersCached(mangaID)
	if err != nil {
		t.Fatalf("ListChaptersCached: %v", err)
	}
	if len(chapters) != 1 {
		t.Fatalf("got %d chapters, want 1", len(chapters))
	}
	got := chapters[0].ReleasedAt
	if got.IsZero() {
		t.Fatal("released_at was lost: got zero time, want 2024-09-07")
	}
	if !got.UTC().Equal(released) {
		t.Errorf("released_at = %s, want %s", got.UTC().Format(time.RFC3339), released.Format(time.RFC3339))
	}
}

// A plugin that knows no date must not have one invented for it: the column
// stays NULL rather than falling back to the sync time.
func TestUpsertChapterWithoutReleaseDateStaysNull(t *testing.T) {
	db := openTestDB(t)

	mangaID, err := db.UpsertManga(Manga{PluginID: "p", SourceMangaID: "m1", Title: "M"})
	if err != nil {
		t.Fatalf("UpsertManga: %v", err)
	}
	if _, err := db.UpsertChapter(Chapter{
		MangaID:         mangaID,
		SourceChapterID: "ch1",
		Title:           "Chapter 1",
		ChapterNum:      1,
	}); err != nil {
		t.Fatalf("UpsertChapter: %v", err)
	}

	var isNull int
	if err := db.db.QueryRow(
		`SELECT released_at IS NULL FROM chapters WHERE manga_id = ? AND source_chapter_id = ?`,
		mangaID, "ch1",
	).Scan(&isNull); err != nil {
		t.Fatalf("query released_at: %v", err)
	}
	if isNull != 1 {
		t.Error("released_at should be NULL when the plugin reports no date")
	}
}

// Re-syncing a chapter must refresh fetched_at (the sync time) without
// discarding the release date the plugin reported.
func TestUpsertChapterResyncKeepsReleaseDate(t *testing.T) {
	db := openTestDB(t)

	mangaID, err := db.UpsertManga(Manga{PluginID: "p", SourceMangaID: "m1", Title: "M"})
	if err != nil {
		t.Fatalf("UpsertManga: %v", err)
	}

	released := time.Date(2023, 5, 1, 12, 0, 0, 0, time.UTC)
	for range 2 {
		if _, err := db.UpsertChapter(Chapter{
			MangaID:         mangaID,
			SourceChapterID: "ch1",
			Title:           "Chapter 1",
			ChapterNum:      1,
			ReleasedAt:      released,
		}); err != nil {
			t.Fatalf("UpsertChapter: %v", err)
		}
	}

	chapters, err := db.ListChaptersCached(mangaID)
	if err != nil {
		t.Fatalf("ListChaptersCached: %v", err)
	}
	if len(chapters) != 1 {
		t.Fatalf("got %d chapters, want 1", len(chapters))
	}
	if !chapters[0].ReleasedAt.UTC().Equal(released) {
		t.Errorf("released_at after re-sync = %s, want %s",
			chapters[0].ReleasedAt.UTC().Format(time.RFC3339), released.Format(time.RFC3339))
	}
	if chapters[0].FetchedAt.IsZero() {
		t.Error("fetched_at must still record the sync time")
	}
}

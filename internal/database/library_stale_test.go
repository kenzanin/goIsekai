package database

import (
	"testing"
	"time"
)

func TestListLibraryStale(t *testing.T) {
	db := openTestDB(t)

	if _, err := db.UpsertManga(Manga{PluginID: "p", SourceMangaID: "fresh", InLibrary: true}); err != nil {
		t.Fatalf("upsert fresh: %v", err)
	}
	// Not in library: never selected even though the cutoff is far future.
	if _, err := db.UpsertManga(Manga{PluginID: "p", SourceMangaID: "ghost", InLibrary: false}); err != nil {
		t.Fatalf("upsert ghost: %v", err)
	}

	// A cutoff in the future makes the freshly stamped rows stale.
	got, err := db.ListLibraryStale(time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("ListLibraryStale: %v", err)
	}
	if len(got) != 1 || got[0].SourceMangaID != "fresh" {
		t.Fatalf("want only [fresh], got %+v", got)
	}

	// A recent cutoff (yesterday) excludes everything stamped today.
	got, err = db.ListLibraryStale(time.Now().AddDate(0, 0, -1))
	if err != nil {
		t.Fatalf("ListLibraryStale recent: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("want none stale, got %+v", got)
	}
}

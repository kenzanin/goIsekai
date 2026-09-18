package database

import (
	"testing"
	"time"
)

func TestListLibraryStale(t *testing.T) {
	db := openTestDB(t)

	old_, _ := time.Parse(time.DateTime, "2000-01-01 00:00:00")

	idOld, err := db.UpsertManga(Manga{PluginID: "p", SourceMangaID: "old", InLibrary: true})
	if err != nil {
		t.Fatalf("upsert old: %v", err)
	}
	// Backdate updated_at directly; the upsert stamps it to now.
	if _, err := db.db.Exec(`UPDATE mangas SET updated_at = ? WHERE id = ?`, old_.UTC().Format(time.DateTime), idOld); err != nil {
		t.Fatalf("backdate: %v", err)
	}
	if _, err := db.UpsertManga(Manga{PluginID: "p", SourceMangaID: "fresh", InLibrary: true}); err != nil {
		t.Fatalf("upsert fresh: %v", err)
	}
	// Not in library: never selected even when ancient.
	if _, err := db.UpsertManga(Manga{PluginID: "p", SourceMangaID: "ghost", InLibrary: false}); err != nil {
		t.Fatalf("upsert ghost: %v", err)
	}
	if _, err := db.db.Exec(`UPDATE mangas SET updated_at = ? WHERE source_manga_id = 'ghost'`, old_.UTC().Format(time.DateTime)); err != nil {
		t.Fatalf("backdate ghost: %v", err)
	}

	got, err := db.ListLibraryStale(time.Now().AddDate(0, 0, -3))
	if err != nil {
		t.Fatalf("ListLibraryStale: %v", err)
	}
	if len(got) != 1 || got[0].SourceMangaID != "old" {
		t.Fatalf("want only [old], got %+v", got)
	}
}

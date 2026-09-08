package database

import (
	"testing"
)

func TestAddAltDescriptionsDedupCount(t *testing.T) {
	db := openTestDB(t)
	if err := db.UpsertManga(Manga{ID: "m1", PluginID: "p1", SourceMangaID: "s1", Title: "Main", InLibrary: true}); err != nil {
		t.Fatalf("upsert manga: %v", err)
	}

	// First batch: 2 distinct + 1 duplicate within the batch.
	n, err := db.AddAltDescriptions("m1", []string{"Short synopsis", "Long synopsis", "Short synopsis"}, "MU")
	if err != nil {
		t.Fatalf("add 1: %v", err)
	}
	if n != 2 {
		t.Fatalf("expected 2 inserted, got %d", n)
	}

	// Re-adding the same descriptions must insert nothing.
	n, err = db.AddAltDescriptions("m1", []string{"Short synopsis", "Long synopsis"}, "MU")
	if err != nil {
		t.Fatalf("add 2: %v", err)
	}
	if n != 0 {
		t.Fatalf("expected 0 inserted on dedup, got %d", n)
	}

	descs, err := db.ListAltDescriptions("m1")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(descs) != 2 {
		t.Fatalf("expected 2 alt descriptions, got %d", len(descs))
	}
	want := []AltDescriptionRow{
		{Description: "Long synopsis", Source: "MU"},
		{Description: "Short synopsis", Source: "MU"},
	}
	for i := range want {
		if descs[i] != want[i] {
			t.Fatalf("desc[%d] = %+v, want %+v", i, descs[i], want[i])
		}
	}
}

func TestRemoveAltDescription(t *testing.T) {
	db := openTestDB(t)
	if err := db.UpsertManga(Manga{ID: "m1", PluginID: "p1", SourceMangaID: "s1", Title: "Main"}); err != nil {
		t.Fatalf("upsert manga: %v", err)
	}
	if _, err := db.AddAltDescriptions("m1", []string{"Synopsis A", "Synopsis B"}, "MU"); err != nil {
		t.Fatalf("add: %v", err)
	}

	if err := db.RemoveAltDescription("m1", "Synopsis A"); err != nil {
		t.Fatalf("remove: %v", err)
	}
	descs, err := db.ListAltDescriptions("m1")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(descs) != 1 || descs[0].Description != "Synopsis B" {
		t.Fatalf("expected only Synopsis B left, got %+v", descs)
	}

	// Removing a non-existent description is a no-op, not an error.
	if err := db.RemoveAltDescription("m1", "Nope"); err != nil {
		t.Fatalf("remove missing: %v", err)
	}
}

func TestAltDescriptionsCascadeDelete(t *testing.T) {
	db := openTestDB(t)
	if err := db.UpsertManga(Manga{ID: "m1", PluginID: "p1", SourceMangaID: "s1", Title: "Main", InLibrary: true}); err != nil {
		t.Fatalf("upsert manga: %v", err)
	}
	if _, err := db.AddAltDescriptions("m1", []string{"Alpha", "Beta"}, "MU"); err != nil {
		t.Fatalf("add alts: %v", err)
	}

	if _, err := db.db.Exec(`DELETE FROM mangas WHERE id = ?`, "m1"); err != nil {
		t.Fatalf("delete manga: %v", err)
	}
	var count int
	if err := db.db.QueryRow(`SELECT COUNT(*) FROM alt_descriptions WHERE manga_row_id = ?`, "m1").Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 alt_descriptions after cascade, got %d", count)
	}
}

func TestMigrationCount(t *testing.T) {
	db := openTestDB(t)

	var version int
	if err := db.db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		t.Fatalf("user_version: %v", err)
	}
	if version != len(migrations) {
		t.Fatalf("user_version=%d, want %d (len(migrations))", version, len(migrations))
	}
}

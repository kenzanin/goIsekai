package database

import (
	"testing"
)

func TestAddAltTitlesDedupCount(t *testing.T) {
	db := openTestDB(t)
	if err := db.UpsertManga(Manga{ID: "m1", PluginID: "p1", SourceMangaID: "s1", Title: "Main", InLibrary: true}); err != nil {
		t.Fatalf("upsert manga: %v", err)
	}

	// First batch: 2 distinct + 1 duplicate within the batch.
	n, err := db.AddAltTitles("m1", []string{"Alpha", "Beta", "Alpha"}, "mal")
	if err != nil {
		t.Fatalf("add 1: %v", err)
	}
	if n != 2 {
		t.Fatalf("expected 2 inserted, got %d", n)
	}

	// Re-adding the same titles must insert nothing.
	n, err = db.AddAltTitles("m1", []string{"Alpha", "Beta"}, "mal")
	if err != nil {
		t.Fatalf("add 2: %v", err)
	}
	if n != 0 {
		t.Fatalf("expected 0 inserted on dedup, got %d", n)
	}

	alts, err := db.ListAltTitles("m1")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(alts) != 2 {
		t.Fatalf("expected 2 alt titles, got %d", len(alts))
	}
	want := []AltTitleRow{{Title: "Alpha", Source: "mal"}, {Title: "Beta", Source: "mal"}}
	for i := range want {
		if alts[i] != want[i] {
			t.Fatalf("alt[%d] = %+v, want %+v", i, alts[i], want[i])
		}
	}
}

func TestRemoveAltTitle(t *testing.T) {
	db := openTestDB(t)
	if err := db.UpsertManga(Manga{ID: "m1", PluginID: "p1", SourceMangaID: "s1", Title: "Main"}); err != nil {
		t.Fatalf("upsert manga: %v", err)
	}
	if _, err := db.AddAltTitles("m1", []string{"Alpha", "Beta"}, "mal"); err != nil {
		t.Fatalf("add: %v", err)
	}

	if err := db.RemoveAltTitle("m1", "Alpha"); err != nil {
		t.Fatalf("remove: %v", err)
	}
	alts, err := db.ListAltTitles("m1")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(alts) != 1 || alts[0].Title != "Beta" {
		t.Fatalf("expected only Beta left, got %+v", alts)
	}

	// Removing a non-existent title is a no-op, not an error.
	if err := db.RemoveAltTitle("m1", "Nope"); err != nil {
		t.Fatalf("remove missing: %v", err)
	}
}

func TestSwapMainTitle(t *testing.T) {
	db := openTestDB(t)
	if err := db.UpsertManga(Manga{ID: "m1", PluginID: "p1", SourceMangaID: "s1", Title: "Old Main", InLibrary: true}); err != nil {
		t.Fatalf("upsert manga: %v", err)
	}
	// newTitle is currently an alt title; it must leave alt_titles on swap.
	if _, err := db.AddAltTitles("m1", []string{"New Main", "Other Alt"}, "mal"); err != nil {
		t.Fatalf("add alts: %v", err)
	}
	if err := db.SyncFTS("m1"); err != nil {
		t.Fatalf("sync fts: %v", err)
	}

	if err := db.SwapMainTitle("p1", "s1", "New Main"); err != nil {
		t.Fatalf("swap: %v", err)
	}

	// Main title updated.
	var title string
	if err := db.db.QueryRow(`SELECT title FROM mangas WHERE id = ?`, "m1").Scan(&title); err != nil {
		t.Fatalf("scan title: %v", err)
	}
	if title != "New Main" {
		t.Fatalf("expected main title New Main, got %q", title)
	}

	// Old main demoted to an alt with source 'user'; promoted title removed.
	alts, err := db.ListAltTitles("m1")
	if err != nil {
		t.Fatalf("list alts: %v", err)
	}
	byTitle := map[string]string{}
	for _, a := range alts {
		byTitle[a.Title] = a.Source
	}
	if byTitle["Other Alt"] != "mal" {
		t.Fatalf("expected Other Alt source mal, got %q", byTitle["Other Alt"])
	}
	if byTitle["Old Main"] != "p1" {
		t.Fatalf("expected Old Main source = plugin id, got %q", byTitle["Old Main"])
	}
	if _, dup := byTitle["New Main"]; dup {
		t.Fatalf("New Main must not remain in alt_titles, got %+v", alts)
	}

	// No-op swap (same main title) must succeed without error.
	if err := db.SwapMainTitle("p1", "s1", "New Main"); err != nil {
		t.Fatalf("re-swap same title: %v", err)
	}
}

func TestSearchLibraryFTSAfterSwap(t *testing.T) {
	db := openTestDB(t)
	if err := db.UpsertManga(Manga{ID: "m1", PluginID: "p1", SourceMangaID: "s1", Title: "Old Main", InLibrary: true}); err != nil {
		t.Fatalf("upsert manga: %v", err)
	}
	if _, err := db.AddAltTitles("m1", []string{"New Main"}, "mal"); err != nil {
		t.Fatalf("add alts: %v", err)
	}
	if err := db.SyncFTS("m1"); err != nil {
		t.Fatalf("sync fts: %v", err)
	}
	if err := db.SwapMainTitle("p1", "s1", "New Main"); err != nil {
		t.Fatalf("swap: %v", err)
	}

	hits, err := db.SearchLibraryFTS("New Main")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(hits) != 1 || hits[0].MangaRowID != "m1" || hits[0].SourceMangaID != "s1" || hits[0].PluginID != "p1" {
		t.Fatalf("expected single hit m1, got %+v", hits)
	}
	if hits[0].Title != "New Main" {
		t.Fatalf("expected hit title New Main, got %q", hits[0].Title)
	}

	// The old main title is now searchable via the alt column.
	hits, err = db.SearchLibraryFTS("Old Main")
	if err != nil {
		t.Fatalf("search old: %v", err)
	}
	if len(hits) != 1 {
		t.Fatalf("expected old title searchable through alt, got %+v", hits)
	}
}

func TestSearchLibraryFTSSanitization(t *testing.T) {
	db := openTestDB(t)
	for _, bad := range []string{`un"balanced`, `(unbalanced`, `unbalanced)`} {
		if _, err := db.SearchLibraryFTS(bad); err == nil {
			t.Fatalf("expected error for query %q, got nil", bad)
		}
	}
	// Empty / blank query returns no results, not an error.
	hits, err := db.SearchLibraryFTS("   ")
	if err != nil {
		t.Fatalf("blank query: %v", err)
	}
	if len(hits) != 0 {
		t.Fatalf("expected no hits for blank query, got %+v", hits)
	}
}

func TestAltTitlesCascadeDelete(t *testing.T) {
	db := openTestDB(t)
	if err := db.UpsertManga(Manga{ID: "m1", PluginID: "p1", SourceMangaID: "s1", Title: "Main", InLibrary: true}); err != nil {
		t.Fatalf("upsert manga: %v", err)
	}
	if _, err := db.AddAltTitles("m1", []string{"Alpha", "Beta"}, "mal"); err != nil {
		t.Fatalf("add alts: %v", err)
	}

	if _, err := db.db.Exec(`DELETE FROM mangas WHERE id = ?`, "m1"); err != nil {
		t.Fatalf("delete manga: %v", err)
	}
	var count int
	if err := db.db.QueryRow(`SELECT COUNT(*) FROM alt_titles WHERE manga_row_id = ?`, "m1").Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 alt_titles after cascade, got %d", count)
	}
}

func TestRebuildLibraryFTS(t *testing.T) {
	db := openTestDB(t)
	if err := db.UpsertManga(Manga{ID: "m1", PluginID: "p1", SourceMangaID: "s1", Title: "Main", InLibrary: true}); err != nil {
		t.Fatalf("upsert manga: %v", err)
	}
	if err := db.UpsertManga(Manga{ID: "m2", PluginID: "p1", SourceMangaID: "s2", Title: "Not In Lib"}); err != nil {
		t.Fatalf("upsert manga 2: %v", err)
	}
	if _, err := db.AddAltTitles("m1", []string{"Alt One", "Alt Two"}, "mal"); err != nil {
		t.Fatalf("add alts: %v", err)
	}

	if err := db.RebuildLibraryFTS(); err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	var count int
	if err := db.db.QueryRow(`SELECT COUNT(*) FROM library_fts`).Scan(&count); err != nil {
		t.Fatalf("fts count: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 fts row (in-library only), got %d", count)
	}

	var alt string
	if err := db.db.QueryRow(`SELECT alt FROM library_fts WHERE manga_row_id = ?`, "m1").Scan(&alt); err != nil {
		t.Fatalf("scan alt: %v", err)
	}
	if alt != "Alt One Alt Two" {
		t.Fatalf("expected rebuilt alt 'Alt One Alt Two', got %q", alt)
	}
}

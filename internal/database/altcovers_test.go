package database

import (
	"fmt"
	"testing"
)

func TestAltCoversDedupAndReset(t *testing.T) {
	db := openTestDB(t)
	mangaID, err := db.UpsertManga(Manga{PluginID: "p1", SourceMangaID: "s1", Title: "Main"})
	if err != nil {
		t.Fatalf("upsert manga: %v", err)
	}
	rowID := fmt.Sprint(mangaID)

	covers := []AltCoverRow{
		{URL: "https://a/1.jpg", Source: "mangadex"},
		{URL: "https://a/2.jpg", Source: "mangadex"},
		{URL: "https://a/1.jpg", Source: "mangaupdates"}, // dedup within batch
		{URL: "", Source: "mangadex"},                    // blank skipped
	}
	n, err := db.AddAltCovers(rowID, covers)
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	if n != 2 {
		t.Fatalf("expected 2 inserted, got %d", n)
	}

	// Re-adding the same URLs must insert nothing.
	n, err = db.AddAltCovers(rowID, covers)
	if err != nil {
		t.Fatalf("re-add: %v", err)
	}
	if n != 0 {
		t.Fatalf("expected 0 on dedup, got %d", n)
	}

	list, err := db.ListAltCovers(rowID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 covers, got %d", len(list))
	}
	if list[0].URL != "https://a/1.jpg" || list[0].Source != "mangadex" {
		t.Fatalf("cover[0] = %+v", list[0])
	}

	// ResetEnrichment clears the candidates too.
	if err := db.ResetEnrichment(rowID); err != nil {
		t.Fatalf("reset: %v", err)
	}
	list, err = db.ListAltCovers(rowID)
	if err != nil {
		t.Fatalf("list after reset: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("expected 0 covers after reset, got %d", len(list))
	}
}

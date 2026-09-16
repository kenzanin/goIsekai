package bridge

import (
	"fmt"
	"testing"

	"goisekai/internal/database"
)

func TestSetMainTitleRejectsUnknownTitle(t *testing.T) {
	s := newTestService(t)

	mangaID, err := s.db.UpsertManga(database.Manga{
		PluginID:      "p2",
		SourceMangaID: "s2",
		Title:         "Original",
		InLibrary:     true,
	})
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if _, err := s.db.AddAltTitles(fmt.Sprint(mangaID), []string{"Known Alt"}, "src"); err != nil {
		t.Fatalf("add alt: %v", err)
	}

	// Unknown title must be rejected.
	if err := s.SetMainTitle("p2", "s2", "Nonexistent"); err == nil {
		t.Fatal("expected error for unknown title")
	}

	// Known title must succeed.
	if err := s.SetMainTitle("p2", "s2", "Known Alt"); err != nil {
		t.Fatalf("SetMainTitle known: %v", err)
	}

	// Verify the swap took effect.
	rowID, err := s.db.MangaRowID("p2", "s2")
	if err != nil {
		t.Fatalf("MangaRowID: %v", err)
	}
	alts, err := s.db.ListAltTitles(rowID)
	if err != nil {
		t.Fatalf("ListAltTitles: %v", err)
	}
	for _, a := range alts {
		if a.Title == "Known Alt" {
			t.Fatal("promoted title must be removed from alt list")
		}
	}
}

func TestSearchLibraryRanksExactAboveSubstring(t *testing.T) {
	s := newTestService(t)

	// Insert two manga with related titles.
	var ids [2]int64
	for i, m := range []database.Manga{
		{PluginID: "exact", SourceMangaID: "1", Title: "Solo Leveling", InLibrary: true},
		{PluginID: "sub", SourceMangaID: "1", Title: "Solo Leveling Ragnarok", InLibrary: true},
	} {
		id, err := s.db.UpsertManga(m)
		if err != nil {
			t.Fatalf("upsert %s: %v", m.SourceMangaID, err)
		}
		ids[i] = id
	}
	if err := s.db.SyncFTS(fmt.Sprint(ids[0])); err != nil {
		t.Fatalf("sync fts 1: %v", err)
	}
	if err := s.db.SyncFTS(fmt.Sprint(ids[1])); err != nil {
		t.Fatalf("sync fts 2: %v", err)
	}

	hits, err := s.SearchLibrary("Solo Leveling")
	if err != nil {
		t.Fatalf("SearchLibrary: %v", err)
	}
	if len(hits) < 2 {
		t.Fatalf("expected at least 2 hits, got %d", len(hits))
	}
	// Exact match must rank above substring match.
	if hits[0].Title != "Solo Leveling" {
		t.Errorf("expected exact match first, got %q (score %d)", hits[0].Title, hits[0].Score)
	}
	if hits[1].Title != "Solo Leveling Ragnarok" {
		t.Errorf("expected substring match second, got %q (score %d)", hits[1].Title, hits[1].Score)
	}
	if hits[0].Score <= hits[1].Score {
		t.Errorf("exact score %d should be > substring score %d", hits[0].Score, hits[1].Score)
	}
}

func TestRemoveAltTitleKeepsFTSInSync(t *testing.T) {
	s := newTestService(t)

	mangaID, err := s.db.UpsertManga(database.Manga{
		PluginID:      "rm",
		SourceMangaID: "1",
		Title:         "Tower of God",
		InLibrary:     true,
	})
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if _, err := s.db.AddAltTitles(fmt.Sprint(mangaID), []string{"Kami no Tou"}, "src"); err != nil {
		t.Fatalf("add alt: %v", err)
	}
	if err := s.db.SyncFTS(fmt.Sprint(mangaID)); err != nil {
		t.Fatalf("sync fts: %v", err)
	}

	// The alt term is searchable before removal.
	hits, err := s.SearchLibrary("Kami no Tou")
	if err != nil {
		t.Fatalf("SearchLibrary before remove: %v", err)
	}
	if len(hits) != 1 || hits[0].SourceMangaID != "1" {
		t.Fatalf("expected alt-term hit before removal, got %+v", hits)
	}

	if err := s.RemoveAltTitle("rm", "1", "Kami no Tou"); err != nil {
		t.Fatalf("RemoveAltTitle: %v", err)
	}

	// After removal the re-synced FTS row must no longer surface the manga
	// via the removed alt term.
	hits, err = s.SearchLibrary("Kami no Tou")
	if err != nil {
		t.Fatalf("SearchLibrary after remove: %v", err)
	}
	if len(hits) != 0 {
		t.Fatalf("expected no hits after removing alt title, got %+v", hits)
	}
}

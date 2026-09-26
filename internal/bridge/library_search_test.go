package bridge

import (
	"path/filepath"
	"testing"

	"goisekai/internal/database"
)

func newSearchService(t *testing.T) *AppService {
	t.Helper()
	db, err := database.Open(filepath.Join(t.TempDir(), "search.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return &AppService{db: db}
}

func seedSearchManga(t *testing.T, s *AppService, pluginID, srcID, title, desc string) {
	t.Helper()
	if _, err := s.db.UpsertManga(database.Manga{
		PluginID: pluginID, SourceMangaID: srcID,
		Title: title, Description: desc, InLibrary: true,
	}); err != nil {
		t.Fatalf("upsert %s: %v", srcID, err)
	}
}

func findHit(hits []SearchHit, srcID string) (SearchHit, bool) {
	for _, h := range hits {
		if h.SourceMangaID == srcID {
			return h, true
		}
	}
	return SearchHit{}, false
}

func TestSearchLibraryRankingTiers(t *testing.T) {
	s := newSearchService(t)
	seedSearchManga(t, s, "p1", "a", "Solo Leveling", "")
	seedSearchManga(t, s, "p1", "b", "The Hero Returns", "An influencer earns money in the real world")
	seedSearchManga(t, s, "p1", "c", "Tower of God", "A hero climbs the tower")

	// Title substring ranks above description-only match; "Solo" absent.
	hits, err := s.SearchLibrary("hero")
	if err != nil {
		t.Fatal(err)
	}
	hitB, okB := findHit(hits, "b")
	hitC, okC := findHit(hits, "c")
	if !okB || !okC {
		t.Fatalf("hero: want b+c hits, got %+v", hits)
	}
	if _, ok := findHit(hits, "a"); ok {
		t.Fatal("hero: 'a' must not match")
	}
	if hitB.Score <= hitC.Score {
		t.Errorf("title hit (b=%d) must outrank description hit (c=%d)", hitB.Score, hitC.Score)
	}
	if hitB.Score < 30 || hitC.Score > 20 {
		t.Errorf("tier bands violated: b=%d (want ≥30), c=%d (want ≤20)", hitB.Score, hitC.Score)
	}

	// Description-only match still surfaces, in the description tier.
	hits, err = s.SearchLibrary("influencer")
	if err != nil {
		t.Fatal(err)
	}
	hitB, okB = findHit(hits, "b")
	if !okB {
		t.Fatalf("influencer: want b via description, got %+v", hits)
	}
	if hitB.Score == 0 || hitB.Score > 20 {
		t.Errorf("influencer: b score=%d, want 1–20", hitB.Score)
	}

	// Partial mid-token keyword matches title.
	hits, err = s.SearchLibrary("olo")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := findHit(hits, "a"); !ok {
		t.Fatalf("olo: want a (mid-token partial), got %+v", hits)
	}
}

func TestSearchLibraryBothMatchKeepsTitleScore(t *testing.T) {
	s := newSearchService(t)
	seedSearchManga(t, s, "p1", "d", "Hero Saga", "A hero story")

	hits, err := s.SearchLibrary("hero")
	if err != nil {
		t.Fatal(err)
	}
	hitD, ok := findHit(hits, "d")
	if !ok {
		t.Fatalf("want d, got %+v", hits)
	}
	if hitD.Score != 80 {
		t.Errorf("both-match keeps title score: got %d, want 80 (prefix=80, desc/5=12)", hitD.Score)
	}
}

func TestSearchLibraryEmptyQuery(t *testing.T) {
	s := newSearchService(t)
	hits, err := s.SearchLibrary("   ")
	if err != nil {
		t.Fatal(err)
	}
	if hits != nil {
		t.Errorf("empty query: want nil, got %+v", hits)
	}
}

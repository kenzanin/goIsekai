package bridge

import (
	"fmt"
	"testing"

	"goisekai/internal/database"
)

// The primary title used throughout: normalization must make these compare equal,
// so a test that passes is testing the alt-title path and not luck.
const (
	candPrimaryTitle = "Karakai Jouzu no Takagi-san"
	candAltTitle     = "Takagi-san, the Cartoonist"
)

func TestMatchKeysCoversAltTitles(t *testing.T) {
	keys := matchKeys(candPrimaryTitle, []string{candAltTitle, "  ", "凡事" + ""})

	if !isExactMatch(candPrimaryTitle, keys) {
		t.Error("the display title should match itself")
	}
	if !isExactMatch(candAltTitle, keys) {
		t.Error("an alternative title should count as an exact match")
	}
	if !isExactMatch("  TAKAGI-SAN,   the Cartoonist!! ", keys) {
		t.Error("an alternative title should match after normalization")
	}
	if isExactMatch("A Different Series Entirely", keys) {
		t.Error("an unrelated title must not match")
	}
	if isExactMatch("", keys) {
		t.Error("a blank candidate title must never match, or it gets auto-selected")
	}
}

func TestMatchKeysWithoutAltTitles(t *testing.T) {
	keys := matchKeys(candPrimaryTitle, nil)
	if !isExactMatch(candPrimaryTitle, keys) {
		t.Error("the display title should match itself")
	}
	if isExactMatch(candAltTitle, keys) {
		t.Error("with no alt titles the alternative title must not match")
	}
}

func TestMatchKeysAllBlankTitles(t *testing.T) {
	if got := len(matchKeys("", []string{"", "   "})); got != 0 {
		t.Errorf("blank titles produced %d keys, want 0 so nothing can match them", got)
	}
}

// An entry that is only reachable under a different name on another source must
// still auto-select. Two candidates matching by different keys stay ambiguous and
// must not auto-select, which is the property that keeps a wrong move impossible.
func TestAutoSelectAcrossAltTitles(t *testing.T) {
	keys := matchKeys(candPrimaryTitle, []string{candAltTitle})
	cands := []MigrationCandidate{
		{PluginID: "a", Title: "Something Else", IsExactMatch: false},
		{PluginID: "b", Title: candAltTitle, IsExactMatch: isExactMatch(candAltTitle, keys)},
	}
	sel := AutoSelectCandidate(cands)
	if sel == nil || sel.PluginID != "b" {
		t.Fatalf("the alt-title candidate should have been auto-selected, got %v", sel)
	}

	keys = matchKeys(candPrimaryTitle, []string{candAltTitle})
	both := []MigrationCandidate{
		{PluginID: "a", Title: candPrimaryTitle, IsExactMatch: isExactMatch(candPrimaryTitle, keys)},
		{PluginID: "b", Title: candAltTitle, IsExactMatch: isExactMatch(candAltTitle, keys)},
	}
	if sel := AutoSelectCandidate(both); sel != nil {
		t.Fatalf("two matching candidates are ambiguous, must not auto-select, got %v", sel)
	}
}

func TestAltTitlesForReadsStoredTitles(t *testing.T) {
	s := newTestService(t)

	mangaID, err := s.db.UpsertManga(database.Manga{
		PluginID:      "kaliscan",
		SourceMangaID: "src-1",
		Title:         candPrimaryTitle,
		InLibrary:     true,
	})
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if _, err := s.db.AddAltTitles(fmt.Sprint(mangaID), []string{candAltTitle, "Ignored"}, "mangaupdates"); err != nil {
		t.Fatalf("add alt titles: %v", err)
	}

	got := s.altTitlesFor("kaliscan", "src-1")
	if len(got) != 2 {
		t.Fatalf("altTitlesFor returned %d titles (%v), want 2", len(got), got)
	}
	for _, want := range []string{candAltTitle, "Ignored"} {
		found := false
		for _, g := range got {
			if g == want {
				found = true
			}
		}
		if !found {
			t.Errorf("altTitlesFor dropped %q, got %v", want, got)
		}
	}
}

// An entry with no alt titles, and an entry that is not in the library at all,
// both have to degrade to "match on the primary title alone" rather than error.
func TestAltTitlesForDegradesQuietly(t *testing.T) {
	s := newTestService(t)

	if got := s.altTitlesFor("kaliscan", "never-added"); len(got) != 0 {
		t.Errorf("unknown manga returned %v, want empty", got)
	}

	mangaID, err := s.db.UpsertManga(database.Manga{
		PluginID: "kaliscan", SourceMangaID: "src-2", Title: candPrimaryTitle,
	})
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if got := s.altTitlesFor("kaliscan", "src-2"); len(got) != 0 {
		t.Errorf("manga with no alt titles returned %v, want empty", got)
	}
	_ = mangaID
}

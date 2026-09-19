package bridge

import (
	"fmt"
	"path/filepath"
	"testing"

	"goisekai/internal/database"
	"goisekai/internal/hostnet"
)

func newGenreTestService(t *testing.T) *AppService {
	t.Helper()
	db, err := database.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return NewAppService(db, nil, hostnet.NewProxy(), "", t.TempDir(), nil)
}

func seedManga(t *testing.T, s *AppService, pluginID, mangaID string) int64 {
	t.Helper()
	id, err := s.db.UpsertManga(database.Manga{PluginID: pluginID, SourceMangaID: mangaID, Title: "T", InLibrary: true})
	if err != nil {
		t.Fatalf("upsert manga: %v", err)
	}
	return id
}

// TestAddGenreKeepsExisting verifies the reported bug: adding a genre when no
// override exists must seed from the enrichment categories (what the panel
// shows), not start an override containing only the new genre.
func TestAddGenreKeepsExisting(t *testing.T) {
	s := newGenreTestService(t)
	rowID := seedManga(t, s, "p", "m")

	// Enrichment stored categories: what the page currently displays.
	if _, err := s.db.AddCategories(fmt.Sprint(rowID), []string{"Action", "Fantasy", "Romance"}, "mangaupdates"); err != nil {
		t.Fatalf("seed categories: %v", err)
	}

	if err := s.ToggleGenre("p", "m", "Horror"); err != nil {
		t.Fatalf("toggle genre: %v", err)
	}
	genres, has, err := s.GetMangaGenres("p", "m")
	if err != nil || !has {
		t.Fatalf("override missing after add: has=%v err=%v", has, err)
	}
	want := map[string]bool{"Action": true, "Fantasy": true, "Romance": true, "Horror": true}
	if len(genres) != len(want) {
		t.Fatalf("genres = %v, want 4 entries %v", genres, want)
	}
	for _, g := range genres {
		if !want[g] {
			t.Errorf("unexpected genre %q in %v", g, genres)
		}
	}
}

// TestRemoveGenreFromPluginList verifies the removal path: toggling off a
// plugin/enrichment genre that was never in the override must write an
// exclusion list (displayed minus that genre), not ADD it.
func TestRemoveGenreFromPluginList(t *testing.T) {
	s := newGenreTestService(t)
	rowID := seedManga(t, s, "p", "m")
	if _, err := s.db.AddCategories(fmt.Sprint(rowID), []string{"Action", "Fantasy", "Romance"}, "mangaupdates"); err != nil {
		t.Fatalf("seed categories: %v", err)
	}

	// ToggleRomance: not in override (none exists) -> currently the code ADDS
	// it. With the fix it seeds displayed then removes, leaving an exclusion
	// list of the remaining genres.
	if err := s.ToggleGenre("p", "m", "Romance"); err != nil {
		t.Fatalf("toggle genre: %v", err)
	}
	genres, has, err := s.GetMangaGenres("p", "m")
	if err != nil {
		t.Fatalf("get genres: %v", err)
	}
	for _, g := range genres {
		if g == "Romance" {
			t.Errorf("removed genre %q still in override %v", g, genres)
		}
	}
	if !has {
		t.Fatalf("expected exclusion override, got none")
	}
}

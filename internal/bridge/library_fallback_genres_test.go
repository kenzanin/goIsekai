package bridge

import (
	"fmt"
	"testing"

	"goisekai/internal/database"
)

// TestCachedMangaFallbackPopulatesGenres covers the detail page's genre tags and
// the enrichment panel's active-chip highlighting.
//
// The cached path used to leave RawGenres nil and set Genres only when a user
// override existed, so a library manga opening from cache rendered no genre tags
// at all and the panel could not mark any chip as active — every chip then
// toggled the wrong way. Plugin genres have no column of their own; the
// enrichment categories are their persisted mirror.
func TestCachedMangaFallbackPopulatesGenres(t *testing.T) {
	s := newTestService(t)

	mangaID, err := s.db.UpsertManga(database.Manga{
		PluginID: "plugin-x", SourceMangaID: "src-1", Title: "Cached Genres", InLibrary: true,
	})
	if err != nil {
		t.Fatalf("upsert manga: %v", err)
	}
	rowID, err := s.db.ResolveMangaRowID("plugin-x", "src-1")
	if err != nil {
		t.Fatalf("resolve row id: %v", err)
	}
	for _, c := range []string{"Action", "Comedy", "Isekai"} {
		if err := s.db.AddCategory(rowID, c); err != nil {
			t.Fatalf("add category %s: %v", c, err)
		}
	}

	// No override: the tags shown and the panel's active set are both the
	// plugin genres mirrored as categories.
	manga, _, err := s.CachedMangaAndChapters("plugin-x", "src-1")
	if err != nil {
		t.Fatalf("CachedMangaAndChapters: %v", err)
	}
	if len(manga.RawGenres) != 3 {
		t.Fatalf("RawGenres should carry the plugin genres, got %v", manga.RawGenres)
	}
	if fmt.Sprint(manga.Genres) != fmt.Sprint(manga.RawGenres) {
		t.Fatalf("without an override Genres should equal the plugin genres: %v vs %v", manga.Genres, manga.RawGenres)
	}

	// Override present: the tags shown become the override, while RawGenres
	// still reports the plugin set so the panel keeps listing every chip.
	if err := s.db.SetMangaGenres(mangaID, []string{"Drama"}); err != nil {
		t.Fatalf("set manga genres: %v", err)
	}
	manga, _, err = s.CachedMangaAndChapters("plugin-x", "src-1")
	if err != nil {
		t.Fatalf("CachedMangaAndChapters after override: %v", err)
	}
	if fmt.Sprint(manga.Genres) != fmt.Sprint([]string{"Drama"}) {
		t.Fatalf("override should win for the displayed tags, got %v", manga.Genres)
	}
	if len(manga.RawGenres) != 3 {
		t.Fatalf("RawGenres must keep the plugin genres under an override, got %v", manga.RawGenres)
	}
}

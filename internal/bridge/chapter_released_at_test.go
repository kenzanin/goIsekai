package bridge

import (
	"path/filepath"
	"testing"
	"time"

	"goisekai/internal/database"
	"goisekai/internal/hostnet"
	"goisekai/pkg/types"
)

// newReleasedAtService returns a service whose plugin is registered but
// inactive, so the chapter list is served from SQLite — the path the detail
// page uses when a plugin is unreachable.
func newReleasedAtService(t *testing.T) *AppService {
	t.Helper()
	db, err := database.Open(filepath.Join(t.TempDir(), "released_at.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.RegisterPlugin(database.Plugin{ID: "rel", Name: "rel", IsActive: false}); err != nil {
		t.Fatalf("register plugin: %v", err)
	}
	s := NewAppService(db, nil, hostnet.NewProxy(), "", "", nil)
	t.Cleanup(s.Shutdown)
	return s
}

// A chapter's release date must survive the trip plugin → database → chapter
// list. Before the fix the column held the sync timestamp, so every chapter
// showed the same day instead of its own upload date.
func TestCachedChaptersKeepReleaseDate(t *testing.T) {
	s := newReleasedAtService(t)

	released := time.Date(2023, 8, 14, 6, 30, 0, 0, time.UTC)
	chapters := []types.Chapter{
		{ID: "ch-1", Title: "Chapter 1", ChapterNum: 1, ReleasedAt: released},
		{ID: "ch-2", Title: "Chapter 2", ChapterNum: 2, ReleasedAt: released.Add(24 * time.Hour)},
	}
	manga := types.Manga{ID: "src-1", Title: "Dated Manga"}
	if err := s.persistMangaDetails("rel", manga, chapters); err != nil {
		t.Fatalf("persistMangaDetails: %v", err)
	}
	// Chapters are only persisted for library manga.
	if err := s.ToggleLibraryItem("rel", "src-1"); err != nil {
		t.Fatalf("ToggleLibraryItem: %v", err)
	}
	// persistMangaDetails ran before the row was in the library, so persist again.
	if err := s.persistMangaDetails("rel", manga, chapters); err != nil {
		t.Fatalf("persistMangaDetails (in library): %v", err)
	}

	_, got, err := s.CachedMangaAndChapters("rel", "src-1")
	if err != nil {
		t.Fatalf("CachedMangaAndChapters: %v", err)
	}
	if len(got) != len(chapters) {
		t.Fatalf("got %d chapters, want %d", len(got), len(chapters))
	}
	byID := map[string]types.Chapter{}
	for _, c := range got {
		byID[c.ID] = c
	}
	for _, want := range chapters {
		c, ok := byID[want.ID]
		if !ok {
			t.Fatalf("chapter %s missing from the cached list", want.ID)
		}
		if c.ReleasedAt.IsZero() {
			t.Errorf("chapter %s: released_at lost (zero time), want %s",
				want.ID, want.ReleasedAt.Format(time.RFC3339))
			continue
		}
		if !c.ReleasedAt.UTC().Equal(want.ReleasedAt) {
			t.Errorf("chapter %s: released_at = %s, want %s",
				want.ID, c.ReleasedAt.UTC().Format(time.RFC3339), want.ReleasedAt.UTC().Format(time.RFC3339))
		}
	}
}

// The inactive-plugin chapter list must report the release date too, not the
// moment the row was last synced.
func TestGetChapterListInactivePluginKeepsReleaseDate(t *testing.T) {
	s := newReleasedAtService(t)

	released := time.Date(2022, 1, 2, 3, 4, 5, 0, time.UTC)
	manga := types.Manga{ID: "src-2", Title: "Another Manga"}
	if err := s.persistMangaDetails("rel", manga, []types.Chapter{
		{ID: "ch-a", Title: "Chapter A", ChapterNum: 1, ReleasedAt: released},
	}); err != nil {
		t.Fatalf("persistMangaDetails: %v", err)
	}
	if err := s.ToggleLibraryItem("rel", "src-2"); err != nil {
		t.Fatalf("ToggleLibraryItem: %v", err)
	}
	if err := s.persistMangaDetails("rel", manga, []types.Chapter{
		{ID: "ch-a", Title: "Chapter A", ChapterNum: 1, ReleasedAt: released},
	}); err != nil {
		t.Fatalf("persistMangaDetails (in library): %v", err)
	}

	got, err := s.GetChapterList("rel", "src-2")
	if err != nil {
		t.Fatalf("GetChapterList: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d chapters, want 1", len(got))
	}
	if !got[0].ReleasedAt.UTC().Equal(released) {
		t.Errorf("released_at = %s, want %s",
			got[0].ReleasedAt.UTC().Format(time.RFC3339), released.UTC().Format(time.RFC3339))
	}
}

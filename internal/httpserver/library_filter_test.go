package httpserver

import (
	"strconv"
	"testing"

	"goisekai/internal/database"
)

// filterBranchFixture: four manga whose read states cover every branch —
// mid-reading (0 < read < total), untouched (read == 0), finished
// (read >= total > 0), and chapter-less (total == 0).
func filterBranchFixture(stats map[string]database.LibraryMangaStats) []database.Manga {
	set := func(id int64, total, read int, hasNew bool) {
		key := strconv.FormatInt(id, 10)
		stats[key] = database.LibraryMangaStats{MangaID: key, TotalChapters: total, ReadChapters: read, HasNew: hasNew}
	}
	set(1, 10, 3, false) // reading
	set(2, 10, 0, false) // unread
	set(3, 10, 10, false)
	set(4, 0, 0, false)  // no chapters
	set(5, 5, 5, true)   // done + new
	return []database.Manga{
		{ID: 1}, {ID: 2}, {ID: 3}, {ID: 4}, {ID: 5},
	}
}

func filterIDs(mangas []database.Manga) []int64 {
	out := make([]int64, 0, len(mangas))
	for _, m := range mangas {
		out = append(out, m.ID)
	}
	return out
}

func TestFilterLibraryStatusBranches(t *testing.T) {
	t.Run("reading keeps only mid-progress", func(t *testing.T) {
		stats := map[string]database.LibraryMangaStats{}
		mangas := filterBranchFixture(stats)
		got := filterIDs(filterLibrary(mangas, stats, nil, "reading", ""))
		want := []int64{1}
		assertInt64s(t, got, want)
	})
	t.Run("unread keeps read==0 and total==0", func(t *testing.T) {
		stats := map[string]database.LibraryMangaStats{}
		mangas := filterBranchFixture(stats)
		got := filterIDs(filterLibrary(mangas, stats, nil, "unread", ""))
		// ID 4 has total==0: readState returns 0/0, so read==0 keeps it.
		want := []int64{2, 4}
		assertInt64s(t, got, want)
	})
	t.Run("done keeps read>=total>0", func(t *testing.T) {
		stats := map[string]database.LibraryMangaStats{}
		mangas := filterBranchFixture(stats)
		got := filterIDs(filterLibrary(mangas, stats, nil, "done", ""))
		want := []int64{3, 5}
		assertInt64s(t, got, want)
	})
	t.Run("new keeps only HasNew", func(t *testing.T) {
		stats := map[string]database.LibraryMangaStats{}
		mangas := filterBranchFixture(stats)
		got := filterIDs(filterLibrary(mangas, stats, nil, "new", ""))
		want := []int64{5}
		assertInt64s(t, got, want)
	})
	t.Run("all keeps everything", func(t *testing.T) {
		stats := map[string]database.LibraryMangaStats{}
		mangas := filterBranchFixture(stats)
		got := filterIDs(filterLibrary(mangas, stats, nil, "all", ""))
		if len(got) != 5 {
			t.Fatalf("all: got %v, want 5 ids", got)
		}
	})
}

func TestHasCategoryCaseInsensitive(t *testing.T) {
	cats := map[string][]string{"7": {"Action", "Sci-Fi"}}
	m := database.Manga{ID: 7}
	if !hasCategory(cats, m, "action") {
		t.Error("lowercased tag should match stored 'Action'")
	}
	if !hasCategory(cats, m, "  SCI-FI  ") {
		t.Error("trimmed+upper tag should match stored 'Sci-Fi'")
	}
	if hasCategory(cats, m, "romance") {
		t.Error("unrelated tag must not match")
	}
	if hasCategory(cats, database.Manga{ID: 8}, "action") {
		t.Error("manga with no category rows must not match")
	}
}

func assertInt64s(t *testing.T, got, want []int64) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

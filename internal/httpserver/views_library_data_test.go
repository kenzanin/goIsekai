package httpserver

import (
	"strconv"
	"testing"

	"goisekai/internal/database"
)

// TestLibraryDupMaps checks the two invariants the grid depends on: every
// member of a group shares that group's colour, and the count is the number of
// manga in any group (not the number of groups).
func TestLibraryDupMaps(t *testing.T) {
	groups := []database.DuplicateGroup{
		{Key: "a", Members: []database.Manga{{ID: 10}, {ID: 11}}},
		{Key: "b", Members: []database.Manga{{ID: 20}, {ID: 21}, {ID: 22}}},
	}

	colors, rank, count := libraryDupMaps(groups)

	if count != 5 {
		t.Errorf("count = %d, want 5 (members, not groups)", count)
	}
	if len(colors) != 5 || len(rank) != 5 {
		t.Fatalf("maps sized %d/%d, want 5/5", len(colors), len(rank))
	}
	for _, member := range []int64{10, 11} {
		id := strconv.FormatInt(member, 10)
		if colors[id] != dupPalette[0] {
			t.Errorf("group 0 member %s colour = %q, want %q", id, colors[id], dupPalette[0])
		}
		if rank[id] != 0 {
			t.Errorf("group 0 member %s rank = %d, want 0", id, rank[id])
		}
	}
	for _, member := range []int64{20, 21, 22} {
		id := strconv.FormatInt(member, 10)
		if colors[id] != dupPalette[1] {
			t.Errorf("group 1 member %s colour = %q, want %q", id, colors[id], dupPalette[1])
		}
		if rank[id] != 1 {
			t.Errorf("group 1 member %s rank = %d, want 1", id, rank[id])
		}
	}
}

// TestLibraryDupMapsEmpty covers the no-duplicates path: the view indexes both
// maps unconditionally, so they must be non-nil even when nothing matched.
func TestLibraryDupMapsEmpty(t *testing.T) {
	colors, rank, count := libraryDupMaps(nil)
	if colors == nil || rank == nil {
		t.Fatal("maps are nil; the view would panic on lookup")
	}
	if count != 0 {
		t.Errorf("count = %d, want 0", count)
	}
}

// TestLibraryPluginCountsSearchGuard documents that the sidebar card is skipped
// for a search result page — that branch returns before touching the service,
// which is why a service-less Server is enough here.
func TestLibraryPluginCountsSearchGuard(t *testing.T) {
	s := &Server{}
	if got := s.libraryPluginCounts("dungeon", nil, nil); got != nil {
		t.Errorf("counts = %v, want nil when a query is set", got)
	}
}

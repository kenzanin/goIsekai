package database

import "testing"

// insertLibraryManga adds one in-library row with the given stored status.
func insertLibraryManga(t *testing.T, db *DB, sourceID, status string) {
	t.Helper()
	if _, err := db.UpsertManga(Manga{
		PluginID:      "p1",
		SourceMangaID: sourceID,
		Title:         sourceID,
		Status:        status,
		InLibrary:     true,
	}); err != nil {
		t.Fatalf("UpsertManga(%s): %v", sourceID, err)
	}
}

// The dashboard buckets compare the stored status label directly: the write
// boundary already normalized it, so there is no spelling left to resolve.
// Completed counts as done, Ongoing and Hiatus both count as ongoing, and
// anything else — Cancelled, Upcoming, empty, or a pre-migration raw word —
// counts as unknown.
func TestLibraryOverviewStatusBuckets(t *testing.T) {
	tests := []struct {
		status     string
		wantDone   int
		wantGoing  int
		wantUnkown int
	}{
		{"Completed", 1, 0, 0},
		{"completed", 1, 0, 0}, // case-insensitive on the stored label
		{"Ongoing", 0, 1, 0},
		{"Hiatus", 0, 1, 0},
		{"Cancelled", 0, 0, 1},
		{"Upcoming", 0, 0, 1},
		{"", 0, 0, 1},
		{"publishing", 0, 0, 1}, // raw pre-migration word: no longer recognized
		{"finished", 0, 0, 1},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			db := openTestDB(t)
			insertLibraryManga(t, db, "s1", tt.status)

			ov, err := db.LibraryOverview()
			if err != nil {
				t.Fatalf("LibraryOverview: %v", err)
			}
			if ov.StatusDone != tt.wantDone {
				t.Errorf("StatusDone = %d, want %d", ov.StatusDone, tt.wantDone)
			}
			if ov.StatusOngoing != tt.wantGoing {
				t.Errorf("StatusOngoing = %d, want %d", ov.StatusOngoing, tt.wantGoing)
			}
			if ov.StatusUnknown != tt.wantUnkown {
				t.Errorf("StatusUnknown = %d, want %d", ov.StatusUnknown, tt.wantUnkown)
			}
		})
	}
}

// TestLibraryOverviewCountsEveryManga pins the aggregation over a mixed library:
// every in-library row lands in exactly one bucket.
func TestLibraryOverviewCountsEveryManga(t *testing.T) {
	db := openTestDB(t)

	statuses := []string{
		"Ongoing", "Ongoing", "Ongoing",
		"Completed", "Completed",
		"Hiatus",
		"Cancelled",
		"Upcoming",
		"publishing", // raw word stored before the vocabulary was enforced
	}
	for i, status := range statuses {
		insertLibraryManga(t, db, "s"+string(rune('a'+i)), status)
	}

	ov, err := db.LibraryOverview()
	if err != nil {
		t.Fatalf("LibraryOverview: %v", err)
	}

	if ov.TotalTitles != len(statuses) {
		t.Errorf("TotalTitles = %d, want %d", ov.TotalTitles, len(statuses))
	}
	if ov.StatusDone != 2 {
		t.Errorf("StatusDone = %d, want 2", ov.StatusDone)
	}
	if ov.StatusOngoing != 4 {
		t.Errorf("StatusOngoing = %d, want 4 (Ongoing×3 + Hiatus)", ov.StatusOngoing)
	}
	if ov.StatusUnknown != 3 {
		t.Errorf("StatusUnknown = %d, want 3 (Cancelled, Upcoming, publishing)", ov.StatusUnknown)
	}
}

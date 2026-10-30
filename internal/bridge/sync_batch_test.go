package bridge

import (
	"testing"
	"time"

	"goisekai/internal/database"
)

// backdateUpdatedAt moves a manga's updated_at into the past so the stale
// selection has something to find. UpsertManga stamps CURRENT_TIMESTAMP, and
// the cutoff comparison runs in SQLite, so the test has to move the row itself.
func backdateUpdatedAt(t *testing.T, db *database.DB, sourceID string, age time.Duration) {
	t.Helper()
	when := time.Now().UTC().Add(-age).Format(time.DateTime)
	if _, err := db.Exec(
		`UPDATE mangas SET updated_at = ? WHERE source_manga_id = ?`, when, sourceID); err != nil {
		t.Fatalf("backdate %s: %v", sourceID, err)
	}
}

// TestListLibraryStaleSelection pins the scheduler's cut: only in-library rows
// older than the cutoff come back. A fresh library row and a non-library row
// must both be left alone, otherwise the hourly pass re-syncs the whole
// library (or worse, re-fetches detail-view cache rows that were never added).
func TestListLibraryStaleSelection(t *testing.T) {
	s := newTestService(t)

	upsertTestManga(t, s.db, "stale", time.Hour)
	upsertTestManga(t, s.db, "fresh", time.Hour)
	if _, err := s.db.UpsertManga(database.Manga{
		PluginID: "p", SourceMangaID: "not-in-library", InLibrary: false,
	}); err != nil {
		t.Fatalf("upsert non-library: %v", err)
	}

	backdateUpdatedAt(t, s.db, "stale", 5*24*time.Hour)
	backdateUpdatedAt(t, s.db, "not-in-library", 5*24*time.Hour)

	cutoff := time.Now().UTC().Add(-3 * 24 * time.Hour)
	got, err := s.db.ListLibraryStale(cutoff)
	if err != nil {
		t.Fatalf("ListLibraryStale: %v", err)
	}

	if len(got) != 1 {
		ids := make([]string, 0, len(got))
		for _, m := range got {
			ids = append(ids, m.SourceMangaID)
		}
		t.Fatalf("stale selection = %v, want exactly [stale]", ids)
	}
	if got[0].SourceMangaID != "stale" {
		t.Errorf("picked %q, want stale", got[0].SourceMangaID)
	}
}

// TestSyncStaleLibraryNoStaleRowsIsNoop covers the early return: with nothing
// stale the pass must not touch the plugin manager (nil here — calling it would
// panic), so this also pins that no plugin work happens when the library is
// fresh.
func TestSyncStaleLibraryNoStaleRowsIsNoop(t *testing.T) {
	s := newTestService(t)
	upsertTestManga(t, s.db, "fresh", time.Hour)

	if err := s.SyncStaleLibrary(time.Now().UTC().Add(-3 * 24 * time.Hour)); err != nil {
		t.Fatalf("SyncStaleLibrary: %v", err)
	}
}

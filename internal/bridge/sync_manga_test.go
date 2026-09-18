package bridge

import (
	"testing"
	"time"

	"goisekai/internal/database"
)

// upsertTestManga inserts a library manga stamped just now; `age` records the
// age the test intends. SyncManga/LibrarySyncState treat the real updated_at
// through the AppService under test.
func upsertTestManga(t *testing.T, db *database.DB, sourceID string, _ time.Duration) {
	t.Helper()
	if _, err := db.UpsertManga(database.Manga{PluginID: "p", SourceMangaID: sourceID, InLibrary: true}); err != nil {
		t.Fatalf("upsert %s: %v", sourceID, err)
	}
}

func TestSyncMangaRefusals(t *testing.T) {
	s := newTestService(t)
	upsertTestManga(t, s.db, "m", time.Hour)

	// Unknown manga: refused before any plugin call.
	if err := s.SyncManga("p", "missing"); err == nil {
		t.Fatal("missing: want error")
	}
	// Not in library: refused (rows are detail-view cache only).
	if _, err := s.db.UpsertManga(database.Manga{PluginID: "p", SourceMangaID: "ghost", InLibrary: false}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if err := s.SyncManga("p", "ghost"); err == nil {
		t.Fatal("ghost: want not-in-library error")
	}
}

func TestLibrarySyncState(t *testing.T) {
	s := newTestService(t)
	upsertTestManga(t, s.db, "m", time.Hour)

	now := time.Now()
	// updated_at is stamped "now" at insert, so a 3-day threshold says fresh.
	ts, _ := s.LibrarySyncState("p", "m", now)
	if ts.IsZero() {
		t.Fatal("want the stamp from the fresh row")
	}
	// The same row goes stale when "now" moves past the threshold.
	if _, stale := s.LibrarySyncState("p", "m", now.AddDate(0, 0, 5)); !stale {
		t.Fatal("row older than threshold: want stale")
	}
	if _, stale := s.LibrarySyncState("p", "missing", now); stale {
		t.Fatal("missing: want not stale for unknown manga")
	}
}

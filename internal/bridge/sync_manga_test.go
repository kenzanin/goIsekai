package bridge

import (
	"errors"
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

func TestSyncMangaTooFresh(t *testing.T) {
	s := newTestService(t)
	upsertTestManga(t, s.db, "fresh", time.Hour)

	err := s.SyncManga("p", "fresh", false)
	if !errors.Is(err, ErrSyncTooFresh) {
		t.Fatalf("want ErrSyncTooFresh, got %v", err)
	}
	// The not-in-library and unknown-manga refusals come before the gate.
	if err := s.SyncManga("p", "missing", false); err == nil || errors.Is(err, ErrSyncTooFresh) {
		t.Fatalf("want unknown-manga error, got %v", err)
	}
}

func TestSyncMangaNotInLibrary(t *testing.T) {
	s := newTestService(t)
	if _, err := s.db.UpsertManga(database.Manga{PluginID: "p", SourceMangaID: "ghost", InLibrary: false}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	err := s.SyncManga("p", "ghost", false)
	if err == nil || errors.Is(err, ErrSyncTooFresh) {
		t.Fatalf("want not-in-library error, got %v", err)
	}
}

func TestSyncMangaStateFreshRow(t *testing.T) {
	s := newTestService(t)
	upsertTestManga(t, s.db, "m", time.Hour)

	now := time.Now()
	// updated_at is stamped "now" at insert, so a 3-day threshold says fresh.
	if _, stale := s.LibrarySyncState("p", "m", now); stale {
		t.Fatal("fresh row: want not stale")
	}
	err := s.SyncManga("p", "m", false)
	if !errors.Is(err, ErrSyncTooFresh) {
		t.Fatalf("want ErrSyncTooFresh, got %v", err)
	}
	// The same row goes stale when "now" moves past the threshold.
	if _, stale := s.LibrarySyncState("p", "m", now.AddDate(0, 0, 5)); !stale {
		t.Fatal("row older than threshold: want stale")
	}
}

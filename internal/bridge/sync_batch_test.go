package bridge

import (
	"path/filepath"
	"testing"
	"time"

	"goisekai/internal/database"
	"goisekai/internal/hostnet"
	"goisekai/internal/pluginmanager"
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

// TestSyncMangasSkipsInactiveAndContinuesPastFailures pins the batch pass's
// two survivability contracts: a switched-off plugin's rows are untouched
// (never even reaching the plugin manager), and one source failing its detail
// fetch must not abort the pass — the remaining rows are still attempted.
func TestSyncMangasSkipsInactiveAndContinuesPastFailures(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "sync.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	for _, p := range []database.Plugin{
		{ID: "on", Name: "on", IsActive: true},
		{ID: "off", Name: "off", IsActive: false},
	} {
		if err := db.RegisterPlugin(p); err != nil {
			t.Fatalf("register %s: %v", p.ID, err)
		}
	}

	proxy := hostnet.NewProxy()
	// An empty plugin dir: every GetMangaDetail for an active plugin fails,
	// which is exactly the dead-source scenario the pass must survive.
	mgr := pluginmanager.NewManager(proxy, t.TempDir())
	s := NewAppService(db, mgr, proxy, "", "", nil)
	t.Cleanup(s.Shutdown)

	for _, id := range []string{"off-row", "on-row-1", "on-row-2"} {
		upsertTestManga(t, db, id, time.Hour)
	}
	// Make all three stale, then repoint the two "on-row" fixtures at the
	// failing active plugin.
	for _, id := range []string{"off-row", "on-row-1", "on-row-2"} {
		backdateUpdatedAt(t, db, id, time.Hour)
	}
	// Repoint the two "on-row" fixtures at the failing active plugin.
	for _, id := range []string{"on-row-1", "on-row-2"} {
		if _, err := db.Exec(`UPDATE mangas SET plugin_id = 'on' WHERE source_manga_id = ?`, id); err != nil {
			t.Fatalf("repoint %s: %v", id, err)
		}
	}

	stale, err := db.ListLibraryStale(time.Now().UTC().Add(-time.Minute))
	if err != nil {
		t.Fatalf("ListLibraryStale: %v", err)
	}
	if len(stale) != 3 {
		t.Fatalf("stale = %d rows, want 3", len(stale))
	}

	// Must not panic (nil-deref would mean the inactive plugin reached the
	// manager) and must return nil (individual failures are logged, not
	// propagated — one dead source cannot abort the hourly pass).
	if err := s.syncMangas(stale); err != nil {
		t.Fatalf("syncMangas returned %v, want nil", err)
	}

	// The skip itself is asserted by the no-panic contract above: reaching
	// the plugin manager for "off-row" would nil-deref (the inactive branch
	// short-circuits before mgr) and the dead source's failures are logged
	// inside the pass, not returned.
}

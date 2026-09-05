package bridge

import (
	"os"
	"path/filepath"
	"testing"

	"goisekai/internal/database"
	"goisekai/internal/hostnet"
	"goisekai/internal/pluginmanager"
	"goisekai/pkg/types"
)

// newTestServiceWithAltTitles sets up an AppService with a real DB, a real
// pluginmanager loaded with a minimal JS alt-title provider, and a hostnet
// proxy. It returns the service and the provider plugin ID used for lookups.
func newTestServiceWithAltTitles(t *testing.T) *AppService {
	t.Helper()
	db, err := database.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	// Write a minimal JS plugin with alt-title-server capability.
	pluginsDir := t.TempDir()
	plugDir := filepath.Join(pluginsDir, "altsrc")
	if err := os.MkdirAll(plugDir, 0o755); err != nil {
		t.Fatalf("mkdir plugin: %v", err)
	}
	mainJS := `var PLUGIN = {
    contract_version: 1,
    name: "Test Alt Source",
    alt_title_servers: [{ id: "testserver", name: "Test Server" }],
};
function searchManga(a){ return "[]"; }
function getMangaDetail(a){ return "{}"; }
function getChapterList(a){ return "[]"; }
function getPageList(a){ return "[]"; }
function getAltTitles(a){
    var inp = JSON.parse(a);
    var title = inp.title || "";
    // Return titles derived from the input title.
    var out = [];
    if (title != "Alpha") out.push("Alpha");
    out.push("Beta");
    out.push("Gamma");
    return JSON.stringify({ source: "TestProvider", titles: out });
}`
	if err := os.WriteFile(filepath.Join(plugDir, "main.js"), []byte(mainJS), 0o644); err != nil {
		t.Fatalf("write main.js: %v", err)
	}

	mgr := pluginmanager.NewManager(hostnet.NewProxy(), pluginsDir)
	if err := mgr.Discover(); err != nil {
		t.Fatalf("Discover: %v", err)
	}
	t.Cleanup(func() { _ = mgr.Close() })

	return NewAppService(db, mgr, hostnet.NewProxy(), "", "")
}

func TestFetchAltTitlesMergeAndDedup(t *testing.T) {
	s := newTestServiceWithAltTitles(t)

	// Seed a manga row directly in the DB.
	if err := s.db.UpsertManga(database.Manga{
		ID:            "altsrc|s",
		PluginID:      "altsrc",
		SourceMangaID: "s",
		Title:         "The Manga",
		InLibrary:     true,
	}); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	// Trigger plugin load so that alt_title_servers metadata is available.
	if _, err := s.SearchManga("altsrc", types.SearchFilter{Query: "dummy"}); err != nil {
		t.Fatalf("trigger load: %v", err)
	}

	// Pre-insert "Beta" so the fetch merges without duplicating.
	if _, err := s.db.AddAltTitles("altsrc|s", []string{"Beta"}, "other"); err != nil {
		t.Fatalf("seed alt: %v", err)
	}

	alts, err := s.FetchAltTitles("altsrc", "s", "testserver")
	if err != nil {
		t.Fatalf("FetchAltTitles: %v", err)
	}
	// Expect: Alpha, Beta, Gamma — Beta deduplicated with "other" source
	// preserved, new Beta from TestProvider ignored.
	if len(alts) != 3 {
		t.Fatalf("expected 3 alts after merge, got %d: %+v", len(alts), alts)
	}
	byTitle := map[string]string{}
	for _, a := range alts {
		byTitle[a.Title] = a.Source
	}
	if byTitle["Alpha"] != "TestProvider" {
		t.Errorf("Alpha source = %q, want TestProvider", byTitle["Alpha"])
	}
	if byTitle["Beta"] != "other" {
		t.Errorf("Beta source = %q, want other (original kept on dedup)", byTitle["Beta"])
	}
	if byTitle["Gamma"] != "TestProvider" {
		t.Errorf("Gamma source = %q, want TestProvider", byTitle["Gamma"])
	}

	// Calling again should be idempotent.
	alts2, err := s.FetchAltTitles("altsrc", "s", "testserver")
	if err != nil {
		t.Fatalf("FetchAltTitles second call: %v", err)
	}
	if len(alts2) != 3 {
		t.Fatalf("expected 3 alts after idempotent call, got %d: %+v", len(alts2), alts2)
	}
}

func TestSetMainTitleRejectsUnknownTitle(t *testing.T) {
	s := newTestService(t)

	if err := s.db.UpsertManga(database.Manga{
		ID:            "p2|s2",
		PluginID:      "p2",
		SourceMangaID: "s2",
		Title:         "Original",
		InLibrary:     true,
	}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if _, err := s.db.AddAltTitles("p2|s2", []string{"Known Alt"}, "src"); err != nil {
		t.Fatalf("add alt: %v", err)
	}

	// Unknown title must be rejected.
	if err := s.SetMainTitle("p2", "s2", "Nonexistent"); err == nil {
		t.Fatal("expected error for unknown title")
	}

	// Known title must succeed.
	if err := s.SetMainTitle("p2", "s2", "Known Alt"); err != nil {
		t.Fatalf("SetMainTitle known: %v", err)
	}

	// Verify the swap took effect.
	rowID, err := s.db.MangaRowID("p2", "s2")
	if err != nil {
		t.Fatalf("MangaRowID: %v", err)
	}
	alts, err := s.db.ListAltTitles(rowID)
	if err != nil {
		t.Fatalf("ListAltTitles: %v", err)
	}
	for _, a := range alts {
		if a.Title == "Known Alt" {
			t.Fatal("promoted title must be removed from alt list")
		}
	}
}

func TestSearchLibraryRanksExactAboveSubstring(t *testing.T) {
	s := newTestService(t)

	// Insert two manga with related titles.
	for _, m := range []database.Manga{
		{ID: "exact|1", PluginID: "exact", SourceMangaID: "1", Title: "Solo Leveling", InLibrary: true},
		{ID: "sub|1", PluginID: "sub", SourceMangaID: "1", Title: "Solo Leveling Ragnarok", InLibrary: true},
	} {
		if err := s.db.UpsertManga(m); err != nil {
			t.Fatalf("upsert %s: %v", m.ID, err)
		}
	}
	if err := s.db.SyncFTS("exact|1"); err != nil {
		t.Fatalf("sync fts 1: %v", err)
	}
	if err := s.db.SyncFTS("sub|1"); err != nil {
		t.Fatalf("sync fts 2: %v", err)
	}

	hits, err := s.SearchLibrary("Solo Leveling")
	if err != nil {
		t.Fatalf("SearchLibrary: %v", err)
	}
	if len(hits) < 2 {
		t.Fatalf("expected at least 2 hits, got %d", len(hits))
	}
	// Exact match must rank above substring match.
	if hits[0].Title != "Solo Leveling" {
		t.Errorf("expected exact match first, got %q (score %d)", hits[0].Title, hits[0].Score)
	}
	if hits[1].Title != "Solo Leveling Ragnarok" {
		t.Errorf("expected substring match second, got %q (score %d)", hits[1].Title, hits[1].Score)
	}
	if hits[0].Score <= hits[1].Score {
		t.Errorf("exact score %d should be > substring score %d", hits[0].Score, hits[1].Score)
	}
}

func TestRemoveAltTitleKeepsFTSInSync(t *testing.T) {
	s := newTestService(t)

	if err := s.db.UpsertManga(database.Manga{
		ID:            "rm|1",
		PluginID:      "rm",
		SourceMangaID: "1",
		Title:         "Tower of God",
		InLibrary:     true,
	}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if _, err := s.db.AddAltTitles("rm|1", []string{"Kami no Tou"}, "src"); err != nil {
		t.Fatalf("add alt: %v", err)
	}
	if err := s.db.SyncFTS("rm|1"); err != nil {
		t.Fatalf("sync fts: %v", err)
	}

	// The alt term is searchable before removal.
	hits, err := s.SearchLibrary("Kami no Tou")
	if err != nil {
		t.Fatalf("SearchLibrary before remove: %v", err)
	}
	if len(hits) != 1 || hits[0].SourceMangaID != "1" {
		t.Fatalf("expected alt-term hit before removal, got %+v", hits)
	}

	if err := s.RemoveAltTitle("rm", "1", "Kami no Tou"); err != nil {
		t.Fatalf("RemoveAltTitle: %v", err)
	}

	// After removal the re-synced FTS row must no longer surface the manga
	// via the removed alt term.
	hits, err = s.SearchLibrary("Kami no Tou")
	if err != nil {
		t.Fatalf("SearchLibrary after remove: %v", err)
	}
	if len(hits) != 0 {
		t.Fatalf("expected no hits after removing alt title, got %+v", hits)
	}
}

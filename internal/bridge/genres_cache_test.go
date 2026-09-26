package bridge

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"goisekai/internal/database"
	"goisekai/internal/hostnet"
	"goisekai/internal/pluginmanager"
)

const genreFixture = `PLUGIN = { contract_version = 1, name = "GenreTest" }
function search_manga(arg) return "[]" end
function get_manga_detail(arg) return host.json.encode({id = "g1", title = "G"}) end
function get_chapter_list(arg) return "[]" end
function get_page_list(arg) return "[]" end
function get_genres(arg)
    return host.json.encode({{name = "scifi", slug = "sci-fi"}, {name = "bl", slug = "bl"}, {name = "Action", slug = "action"}})
end
`

// writeGenreFixture drops a tiny Lua plugin exporting get_genres into
// pluginsDir/<id>/main.lua and discovers it.
func writeGenreFixture(t *testing.T, id, body string) (*database.DB, *pluginmanager.Manager) {
	t.Helper()
	pluginsDir := t.TempDir()
	pluginDir := filepath.Join(pluginsDir, id)
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatalf("mkdir plugin: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pluginDir, "main.lua"), []byte(body), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	db, err := database.Open(filepath.Join(t.TempDir(), "genres.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.RegisterPlugin(database.Plugin{ID: id, Name: id, IsActive: true}); err != nil {
		t.Fatalf("register plugin: %v", err)
	}
	mgr := pluginmanager.NewManager(hostnet.NewProxy(), pluginsDir)
	if err := mgr.Discover(); err != nil {
		t.Fatalf("Discover: %v", err)
	}
	return db, mgr
}

// TestListGenresFirstCallNormalizesAndPersists: the first ListGenres call
// fetches from the plugin, rewrites alias spellings to canonical names
// ("scifi" → "Sci-Fi", "bl" → "Boys' Love"), keeps slugs verbatim, and
// persists the normalized list to plugins.genres.
func TestListGenresFirstCallNormalizesAndPersists(t *testing.T) {
	db, mgr := writeGenreFixture(t, "genretest", genreFixture)
	s := NewAppService(db, mgr, hostnet.NewProxy(), "", "", nil)

	got, err := s.ListGenres("genretest")
	if err != nil {
		t.Fatalf("ListGenres: %v", err)
	}
	want := []Genre{{Name: "Sci-Fi", Slug: "sci-fi"}, {Name: "Boys' Love", Slug: "bl"}, {Name: "Action", Slug: "action"}}
	if len(got) != len(want) {
		t.Fatalf("got %d genres %v, want %d", len(got), got, len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("genre[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}

	raw, found, err := db.GetPluginGenres("genretest")
	if err != nil || !found {
		t.Fatalf("cache read after first call: found=%v err=%v", found, err)
	}
	var stored []Genre
	if err := json.Unmarshal([]byte(raw), &stored); err != nil {
		t.Fatalf("stored cache not JSON: %v", err)
	}
	if len(stored) != 3 || stored[0].Name != "Sci-Fi" || stored[0].Slug != "sci-fi" {
		t.Errorf("stored cache = %+v, want normalized names + verbatim slugs", stored)
	}
}

// TestListGenresCacheHitSkipsPluginManager: a seeded cache is served without
// touching the plugin manager at all — proven with a nil manager, which would
// panic on any fetch attempt.
func TestListGenresCacheHitSkipsPluginManager(t *testing.T) {
	db, _ := writeGenreFixture(t, "genretest", genreFixture)
	if err := db.SetPluginGenres("genretest", `[{"name":"Action","slug":"action"}]`); err != nil {
		t.Fatalf("seed cache: %v", err)
	}
	s := NewAppService(db, nil, hostnet.NewProxy(), "", "", nil)

	got, err := s.ListGenres("genretest")
	if err != nil {
		t.Fatalf("ListGenres: %v", err)
	}
	if len(got) != 1 || got[0].Name != "Action" || got[0].Slug != "action" {
		t.Fatalf("got %+v, want the seeded cache", got)
	}
}

// TestListGenresNoExportPersistsEmpty: a plugin without get_genres caches "[]"
// so it is never invoked again — the second call must hit the cache (nil
// manager proves it) and return an empty list.
func TestListGenresNoExportPersistsEmpty(t *testing.T) {
	db, mgr := writeGenreFixture(t, "nogenre", `PLUGIN = { contract_version = 1, name = "NoGenre" }
function search_manga(arg) return "[]" end
function get_manga_detail(arg) return host.json.encode({id = "n1", title = "N"}) end
function get_chapter_list(arg) return "[]" end
function get_page_list(arg) return "[]" end
`)
	s := NewAppService(db, mgr, hostnet.NewProxy(), "", "", nil)

	got, err := s.ListGenres("nogenre")
	if err != nil {
		t.Fatalf("ListGenres: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("got %+v, want empty list", got)
	}
	raw, found, err := db.GetPluginGenres("nogenre")
	if err != nil || !found || raw != "[]" {
		t.Fatalf("cache = %q found=%v err=%v, want \"[]\"", raw, found, err)
	}

	s2 := NewAppService(db, nil, hostnet.NewProxy(), "", "", nil)
	got2, err := s2.ListGenres("nogenre")
	if err != nil {
		t.Fatalf("second ListGenres: %v", err)
	}
	if len(got2) != 0 {
		t.Fatalf("second call got %+v, want empty (cache hit)", got2)
	}
}

package pluginmanager

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"goisekai/internal/hostnet"
)

// writeMeta overrides the plugin.json of a folder written by writePluginFolder,
// so a test can pin exact metadata (site_url, timeout, ...).
func writeMeta(t *testing.T, root, id, metaJSON string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, id, "plugin.json"), []byte(metaJSON), 0o644); err != nil {
		t.Fatalf("write plugin.json: %v", err)
	}
}

// SiteURL answers from plugin.json metadata, which is available at discovery
// time — before any runtime is instantiated. That is what lets the host set a
// Referer for gated image CDNs without a plugin invoke.
func TestSiteURLReadsDeclaredMetadata(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writePluginFolder(t, dir, "sited", "main.lua", `
function search(a) return "[]" end
function get_manga_detail(a) return "{}" end
function get_chapter_list(a) return "[]" end
function get_page_list(a) return "[]" end
`)
	writeMeta(t, dir, "sited", `{"contract_version":1,"name":"Sited","site_url":"https://sited.example"}`)

	mgr := NewManager(hostnet.NewProxy(), dir)
	if err := mgr.Discover(); err != nil {
		t.Fatalf("Discover: %v", err)
	}
	defer func() { _ = mgr.Close() }()

	if got := mgr.SiteURL("sited"); got != "https://sited.example" {
		t.Fatalf("SiteURL = %q, want https://sited.example", got)
	}
	// Unknown plugin declares nothing, so it answers empty rather than panicking.
	if got := mgr.SiteURL("nope"); got != "" {
		t.Fatalf("SiteURL(unknown) = %q, want empty", got)
	}
	// Nil-safe: test services run without a manager.
	var nilMgr *Manager
	if got := nilMgr.SiteURL("sited"); got != "" {
		t.Fatalf("SiteURL on nil manager = %q, want empty", got)
	}
}

// GetGenres is an optional export: a plugin without it returns nil, nil so
// callers treat genre browsing as unsupported instead of surfacing an error.
func TestGetGenresReturnsNilWhenExportAbsent(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writePluginFolder(t, dir, "nogenre", "main.lua", `
function search(a) return "[]" end
function get_manga_detail(a) return "{}" end
function get_chapter_list(a) return "[]" end
function get_page_list(a) return "[]" end
`)
	mgr := NewManager(hostnet.NewProxy(), dir)
	if err := mgr.Discover(); err != nil {
		t.Fatalf("Discover: %v", err)
	}
	defer func() { _ = mgr.Close() }()

	genres, err := mgr.GetGenres("nogenre")
	if err != nil {
		t.Fatalf("absent GetGenres must not error: %v", err)
	}
	if len(genres) != 0 {
		t.Fatalf("absent GetGenres = %+v, want none", genres)
	}
}

// When the export is present the host parses its JSON array of {name, slug}.
func TestGetGenresParsesExport(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writePluginFolder(t, dir, "withgenre", "main.lua", `
function search(a) return "[]" end
function get_manga_detail(a) return "{}" end
function get_chapter_list(a) return "[]" end
function get_page_list(a) return "[]" end
function get_genres(a) return '[{"name":"Action","slug":"action"},{"name":"Sci-Fi","slug":"sci-fi"}]' end
`)
	mgr := NewManager(hostnet.NewProxy(), dir)
	if err := mgr.Discover(); err != nil {
		t.Fatalf("Discover: %v", err)
	}
	defer func() { _ = mgr.Close() }()

	genres, err := mgr.GetGenres("withgenre")
	if err != nil {
		t.Fatalf("GetGenres: %v", err)
	}
	if len(genres) != 2 || genres[0].Name != "Action" || genres[0].Slug != "action" {
		t.Fatalf("unexpected genres: %+v", genres)
	}
}

// An unknown plugin id is a real error, not the "unsupported" empty answer.
func TestGetGenresUnknownPluginErrors(t *testing.T) {
	t.Parallel()

	mgr := NewManager(hostnet.NewProxy(), t.TempDir())
	defer func() { _ = mgr.Close() }()

	if _, err := mgr.GetGenres("ghost"); err == nil {
		t.Fatal("GetGenres on an unknown plugin must error")
	}
}

// EnsureLoaded is the public force-load used by the human-verify gate: it must
// populate metadata and report failure for an id that does not exist.
func TestEnsureLoadedPopulatesMetaAndRejectsUnknown(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writePluginFolder(t, dir, "ensured", "main.lua", `
function search(a) return "[]" end
function get_manga_detail(a) return "{}" end
function get_chapter_list(a) return "[]" end
function get_page_list(a) return "[]" end
`)
	writeMeta(t, dir, "ensured", `{"contract_version":1,"name":"Ensured","site_url":"https://ensured.example"}`)

	mgr := NewManager(hostnet.NewProxy(), dir)
	if err := mgr.Discover(); err != nil {
		t.Fatalf("Discover: %v", err)
	}
	defer func() { _ = mgr.Close() }()

	if err := mgr.EnsureLoaded("ensured"); err != nil {
		t.Fatalf("EnsureLoaded: %v", err)
	}
	if got := mgr.SiteURL("ensured"); got != "https://ensured.example" {
		t.Fatalf("after EnsureLoaded SiteURL = %q", got)
	}
	if err := mgr.EnsureLoaded("ghost"); err == nil {
		t.Fatal("EnsureLoaded on an unknown plugin must error")
	}
}

// SetCacheTTL is applied on config reload; it must take effect on the manager.
func TestSetCacheTTLApplies(t *testing.T) {
	t.Parallel()

	mgr := NewManager(hostnet.NewProxy(), t.TempDir())
	defer func() { _ = mgr.Close() }()

	mgr.SetCacheTTL(90 * time.Minute)
	if mgr.cacheTTL != 90*time.Minute {
		t.Fatalf("cacheTTL = %v, want 90m", mgr.cacheTTL)
	}
}

// Proxy exposes the wired proxy for test/debug callers.
func TestProxyReturnsWiredProxy(t *testing.T) {
	t.Parallel()

	p := hostnet.NewProxy()
	mgr := NewManager(p, t.TempDir())
	defer func() { _ = mgr.Close() }()

	if mgr.Proxy() != p {
		t.Fatal("Proxy() must return the proxy the manager was built with")
	}
}

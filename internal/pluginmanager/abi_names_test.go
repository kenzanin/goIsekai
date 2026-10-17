package pluginmanager

import (
	"os"
	"path/filepath"
	"testing"

	"goisekai/internal/hostnet"
	"goisekai/pkg/types"
)

// writePluginFolder lays down a minimal plugin with plugin.json metadata.
func writePluginFolder(t *testing.T, root, id, entry, code string) {
	t.Helper()
	dir := filepath.Join(root, id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, entry), []byte(code), 0o644); err != nil {
		t.Fatalf("write %s: %v", entry, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "plugin.json"),
		[]byte(`{"contract_version":1,"name":"Alias"}`), 0o644); err != nil {
		t.Fatalf("write plugin.json: %v", err)
	}
}

// The ABI is normalised on the host name in each language's case convention:
// Search is `search` in Lua and JS. Both must resolve.
func TestABIUsesCanonicalName(t *testing.T) {
	t.Parallel()

	luaCode := `
function search(a) return '[{"id":"C1","title":"canonical lua"}]' end
function get_manga_detail(a) return '{"id":"C1"}' end
function get_chapter_list(a) return '[]' end
function get_page_list(a) return '[]' end
`
	jsCode := `
function search(a) { return JSON.stringify([{id:"C1",title:"canonical js"}]); }
function getMangaDetail(a) { return JSON.stringify({id:"C1"}); }
function getChapterList(a) { return "[]"; }
function getPageList(a) { return "[]"; }
`
	for _, tc := range []struct {
		name  string
		id    string
		entry string
		code  string
	}{
		{"lua", "canonlua", "main.lua", luaCode},
		{"js", "canonjs", "main.js", jsCode},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writePluginFolder(t, dir, tc.id, tc.entry, tc.code)

			mgr := NewManager(hostnet.NewProxy(), dir)
			if err := mgr.Discover(); err != nil {
				t.Fatalf("Discover: %v", err)
			}
			defer func() { _ = mgr.Close() }()

			mangas, err := mgr.Search(tc.id, types.SearchFilter{Query: "x"})
			if err != nil {
				t.Fatalf("Search via canonical name: %v", err)
			}
			if len(mangas) != 1 || mangas[0].ID != "C1" {
				t.Fatalf("unexpected result: %+v", mangas)
			}
		})
	}
}

// Renaming the ABI must not break the plugins already in the wild: a plugin
// that still exports the pre-rename name keeps loading and running.
func TestABIStillAcceptsLegacyName(t *testing.T) {
	t.Parallel()

	luaCode := `
function search_manga(a) return '[{"id":"L1","title":"legacy lua"}]' end
function get_manga_detail(a) return '{"id":"L1"}' end
function get_chapter_list(a) return '[]' end
function get_page_list(a) return '[]' end
`
	jsCode := `
function searchManga(a) { return JSON.stringify([{id:"L1",title:"legacy js"}]); }
function getMangaDetail(a) { return JSON.stringify({id:"L1"}); }
function getChapterList(a) { return "[]"; }
function getPageList(a) { return "[]"; }
`
	for _, tc := range []struct {
		name  string
		id    string
		entry string
		code  string
	}{
		{"lua", "legacylua", "main.lua", luaCode},
		{"js", "legacyjs", "main.js", jsCode},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writePluginFolder(t, dir, tc.id, tc.entry, tc.code)

			mgr := NewManager(hostnet.NewProxy(), dir)
			if err := mgr.Discover(); err != nil {
				t.Fatalf("Discover: %v", err)
			}
			defer func() { _ = mgr.Close() }()

			mangas, err := mgr.Search(tc.id, types.SearchFilter{Query: "x"})
			if err != nil {
				t.Fatalf("Search via legacy name must keep working: %v", err)
			}
			if len(mangas) != 1 || mangas[0].ID != "L1" {
				t.Fatalf("unexpected result: %+v", mangas)
			}
		})
	}
}

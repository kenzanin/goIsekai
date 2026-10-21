package pluginmanager

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"goisekai/internal/hostnet"
	"goisekai/pkg/types"
)

// writeParamsPlugin lays down a plugin folder with a plugin.json that declares
// params, plus the given source.
func writeParamsPlugin(t *testing.T, root, id, entry, code, meta string) string {
	t.Helper()
	dir := filepath.Join(root, id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, entry), []byte(code), 0o644); err != nil {
		t.Fatalf("write %s: %v", entry, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "plugin.json"), []byte(meta), 0o644); err != nil {
		t.Fatalf("write plugin.json: %v", err)
	}
	return dir
}

// A plugin reads the values it declared in its own plugin.json.
//
// The point of the params block is that a fact which changes (the site URL, a
// CDN host) lives in data the user can edit, not in the plugin's code. A
// plugin that cannot read its own params is the whole feature missing.
func TestPluginReadsItsOwnParams(t *testing.T) {
	t.Parallel()

	const luaCode = `
function search(a) return '[{"id":"x","title":"' .. host.plugin.param("base") .. '"}]' end
function get_manga_detail(a) return '{"id":"x"}' end
function get_chapter_list(a) return '[]' end
function get_page_list(a) return '[]' end
`
	const jsCode = `
function search(a) { return JSON.stringify([{id:"x",title:host.plugin.param("base")}]); }
function getMangaDetail(a) { return JSON.stringify({id:"x"}); }
function getChapterList(a) { return "[]"; }
function getPageList(a) { return "[]"; }
`

	for _, tc := range []struct {
		name  string
		id    string
		entry string
		code  string
	}{
		{"lua", "paramslua", "main.lua", luaCode},
		{"js", "paramsjs", "main.js", jsCode},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			writeParamsPlugin(t, dir, tc.id, tc.entry, tc.code,
				`{"contract_version":1,"name":"Params","params":{"base":"https://example.test"}}`)

			mgr := NewManager(hostnet.NewProxy(), dir)
			if err := mgr.Discover(); err != nil {
				t.Fatalf("Discover: %v", err)
			}
			out, err := mgr.Search(tc.id, types.SearchFilter{Query: "anything"})
			if err != nil {
				t.Fatalf("Search: %v", err)
			}
			if len(out) != 1 {
				t.Fatalf("got %d results, want 1", len(out))
			}
			if out[0].Title != "https://example.test" {
				t.Errorf("title = %q, want the param value", out[0].Title)
			}
		})
	}
}

// The params are readable at LOAD time, not only during a call: a plugin
// builds its URLs at the top level, before any call happens.
func TestPluginReadsParamsAtLoadTime(t *testing.T) {
	t.Parallel()

	const code = `
BASE = host.plugin.param("base")
function search(a) return '[{"id":"x","title":"' .. BASE .. '"}]' end
function get_manga_detail(a) return '{"id":"x"}' end
function get_chapter_list(a) return '[]' end
function get_page_list(a) return '[]' end
`
	dir := t.TempDir()
	writeParamsPlugin(t, dir, "paramsload", "main.lua", code,
		`{"contract_version":1,"name":"Params","params":{"base":"https://loadtime.test"}}`)

	mgr := NewManager(hostnet.NewProxy(), dir)
	if err := mgr.Discover(); err != nil {
		t.Fatalf("Discover: %v", err)
	}
	out, err := mgr.Search("paramsload", types.SearchFilter{Query: "anything"})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(out) != 1 || out[0].Title != "https://loadtime.test" {
		t.Fatalf("got %+v, want the param read at load time", out)
	}
}

// The site_url a plugin already declares is reachable as a param too, so a
// URL is written once in plugin.json instead of twice (once in plugin.json,
// once in the source).
func TestSiteURLIsReadableAsAParam(t *testing.T) {
	t.Parallel()

	const code = `
function search(a) return '[{"id":"x","title":"' .. host.plugin.param("site_url") .. '"}]' end
function get_manga_detail(a) return '{"id":"x"}' end
function get_chapter_list(a) return '[]' end
function get_page_list(a) return '[]' end
`
	dir := t.TempDir()
	writeParamsPlugin(t, dir, "paramssite", "main.lua", code,
		`{"contract_version":1,"name":"Params","site_url":"https://site.test"}`)

	mgr := NewManager(hostnet.NewProxy(), dir)
	if err := mgr.Discover(); err != nil {
		t.Fatalf("Discover: %v", err)
	}
	out, err := mgr.Search("paramssite", types.SearchFilter{Query: "anything"})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(out) != 1 || out[0].Title != "https://site.test" {
		t.Fatalf("got %+v, want the declared site_url", out)
	}
}

// A param the plugin.json does not declare reads as an empty string, not an
// error: a plugin that asks for an optional value must still run.
func TestUnknownParamReadsAsEmptyString(t *testing.T) {
	t.Parallel()

	const code = `
function search(a) return '[{"id":"x","title":"[' .. host.plugin.param("nope") .. ']"}]' end
function get_manga_detail(a) return '{"id":"x"}' end
function get_chapter_list(a) return '[]' end
function get_page_list(a) return '[]' end
`
	dir := t.TempDir()
	writeParamsPlugin(t, dir, "paramsmissing", "main.lua", code,
		`{"contract_version":1,"name":"Params","params":{"base":"https://x.test"}}`)

	mgr := NewManager(hostnet.NewProxy(), dir)
	if err := mgr.Discover(); err != nil {
		t.Fatalf("Discover: %v", err)
	}
	out, err := mgr.Search("paramsmissing", types.SearchFilter{Query: "anything"})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(out) != 1 || out[0].Title != "[]" {
		t.Fatalf("got %+v, want an empty string for an undeclared param", out)
	}
}

// Two plugins must not see each other's params: the value belongs to the
// plugin that declared it.
func TestParamsAreScopedToTheirOwnPlugin(t *testing.T) {
	t.Parallel()

	const code = `
function search(a) return '[{"id":"x","title":"' .. host.plugin.param("base") .. '"}]' end
function get_manga_detail(a) return '{"id":"x"}' end
function get_chapter_list(a) return '[]' end
function get_page_list(a) return '[]' end
`
	dir := t.TempDir()
	writeParamsPlugin(t, dir, "paramsone", "main.lua", code,
		`{"contract_version":1,"name":"One","params":{"base":"https://one.test"}}`)
	writeParamsPlugin(t, dir, "paramstwo", "main.lua", code,
		`{"contract_version":1,"name":"Two","params":{"base":"https://two.test"}}`)

	mgr := NewManager(hostnet.NewProxy(), dir)
	if err := mgr.Discover(); err != nil {
		t.Fatalf("Discover: %v", err)
	}
	for id, want := range map[string]string{
		"paramsone": "https://one.test",
		"paramstwo": "https://two.test",
	} {
		out, err := mgr.Search(id, types.SearchFilter{Query: "anything"})
		if err != nil {
			t.Fatalf("Search %s: %v", id, err)
		}
		if len(out) != 1 || out[0].Title != want {
			t.Errorf("%s got %+v, want %q", id, out, want)
		}
	}
}

// wasm and native reach every helper through ONE dispatcher (env.host_call in
// wasm, the plugin_init callback in native), so plugin.param must resolve there
// too. Both know the plugin id, which is what makes the per-plugin answer work.
func TestWasmAndNativeDispatchPluginParam(t *testing.T) {
	t.Parallel()

	mgr := NewManager(hostnet.NewProxy(), t.TempDir())
	mgr.setParams("wasmplug", types.PluginMeta{
		Params: types.PluginParams{"cdn": "https://cdn.example.test"},
	})

	got, err := mgr.dispatchHostCall(context.Background(), "wasmplug", "plugin.param", []string{"cdn"})
	if err != nil {
		t.Fatalf("dispatchHostCall: %v", err)
	}
	if got != "https://cdn.example.test" {
		t.Errorf("plugin.param(cdn) = %v, want the declared value", got)
	}

	// A plugin that declared nothing, and a param nobody declared, both read
	// as "" rather than an error.
	if got, err := mgr.dispatchHostCall(context.Background(), "otherplug", "plugin.param", []string{"cdn"}); err != nil || got != "" {
		t.Errorf("unknown plugin = (%v, %v), want (\"\", nil)", got, err)
	}
	if got, err := mgr.dispatchHostCall(context.Background(), "wasmplug", "plugin.param", []string{"nope"}); err != nil || got != "" {
		t.Errorf("unknown param = (%v, %v), want (\"\", nil)", got, err)
	}
}

// An interpreted Go plugin reads its params through hostnet.Param, the
// per-plugin bridge function that carries the plugin id like Get and Post do.
func TestYaegiParamReadsItsOwnPluginJSON(t *testing.T) {
	t.Parallel()

	const code = `package main

import "hostnet"

func Search(arg string) (string, error) {
	return ` + "`" + `[{"id":"x","title":"` + "`" + ` + hostnet.Param("base") + ` + "`" + `"}]` + "`" + `, nil
}

func GetMangaDetail(arg string) (string, error) { return ` + "`" + `{"id":"x"}` + "`" + `, nil }
func GetChapterList(arg string) (string, error) { return ` + "`" + `[]` + "`" + `, nil }
func GetPageList(arg string) (string, error)    { return ` + "`" + `[]` + "`" + `, nil }
`
	dir := t.TempDir()
	writeParamsPlugin(t, dir, "paramsyaegi", "main.go", code,
		`{"contract_version":1,"name":"Params","params":{"base":"https://yaegi.test"}}`)

	mgr := NewManager(hostnet.NewProxy(), dir)
	if err := mgr.Discover(); err != nil {
		t.Fatalf("Discover: %v", err)
	}
	out, err := mgr.Search("paramsyaegi", types.SearchFilter{Query: "anything"})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(out) != 1 || out[0].Title != "https://yaegi.test" {
		t.Fatalf("got %+v, want the param read through hostnet.Param", out)
	}
}

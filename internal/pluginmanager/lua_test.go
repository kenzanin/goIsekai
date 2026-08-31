package pluginmanager

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	lua "github.com/yuin/gopher-lua"

	"goisekai/internal/hostnet"
	"goisekai/pkg/types"
)

// writeFileForTest is a tiny helper for writing fixture overrides.
func writeFileForTest(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o644)
}

// luaPluginsDir copies the valid Lua fixture (main.lua + sibling.lua) into a
// temp plugins dir and returns the dir.
func luaPluginsDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	dst := filepath.Join(dir, "luatest")
	if err := copyDir("testdata/luatest", dst); err != nil {
		t.Fatalf("copy fixture: %v", err)
	}
	return dir
}

func TestLuaPluginDiscoveryAndSearch(t *testing.T) {
	dir := luaPluginsDir(t)
	mgr := NewManager(hostnet.NewProxy(), dir)
	if err := mgr.Discover(); err != nil {
		t.Fatalf("Discover: %v", err)
	}
	defer func() { _ = mgr.Close() }()

	mangas, err := mgr.Search("luatest", types.SearchFilter{Query: "test"})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(mangas) != 1 || mangas[0].ID != "L1" || mangas[0].Title != "Lua test" {
		t.Fatalf("unexpected Search result: %+v", mangas)
	}

	detail, err := mgr.GetMangaDetail("luatest", "m1")
	if err != nil {
		t.Fatalf("GetMangaDetail: %v", err)
	}
	if detail.Title != "Detail m1" {
		t.Fatalf("unexpected detail: %+v", detail)
	}

	chapters, err := mgr.GetChapterList("luatest", "m1")
	if err != nil {
		t.Fatalf("GetChapterList: %v", err)
	}
	if len(chapters) != 1 || chapters[0].ChapterNum != 1.0 || chapters[0].MangaID != "m1" {
		t.Fatalf("unexpected chapters: %+v", chapters)
	}

	pages, err := mgr.GetPageList("luatest", "m1/c1")
	if err != nil {
		t.Fatalf("GetPageList: %v", err)
	}
	if len(pages) != 1 || pages[0].URL != "https://example.com/img/1.png" {
		t.Fatalf("unexpected pages: %+v", pages)
	}

	// Meta from the PLUGIN table must survive into PluginMeta.
	p := mgr.plugins["luatest"]
	if p.meta.VerifyURL != "https://example.com" || !p.meta.NeedsHumanVerify || p.meta.ThumbRatio != 0.7 {
		t.Fatalf("unexpected meta: %+v", p.meta)
	}
	if p.kind != "lua" {
		t.Fatalf("kind = %q, want lua", p.kind)
	}
}

func TestLuaPluginRequireSandbox(t *testing.T) {
	dir := t.TempDir()
	// main.lua escapes its folder via a relative require — must fail to load.
	src := `PLUGIN = {contract_version = 1}
function search_manga(a) return "[]" end
function get_manga_detail(a) return "{}" end
function get_chapter_list(a) return "[]" end
function get_page_list(a) return "[]" end
local x = require("../evil")
`
	if err := copyDir("testdata/luatest", filepath.Join(dir, "luatest")); err != nil {
		t.Fatal(err)
	}
	if err := writeFileForTest(filepath.Join(dir, "luatest", "main.lua"), src); err != nil {
		t.Fatal(err)
	}
	evil := filepath.Join(dir, "evil.lua")
	if err := writeFileForTest(evil, "return {}"); err != nil {
		t.Fatal(err)
	}

	mgr := NewManager(hostnet.NewProxy(), dir)
	err := mgr.Discover()
	if err == nil {
		_ = mgr.Close()
		t.Fatal("Discover must fail when main.lua requires outside its folder")
	}
	if !strings.Contains(err.Error(), "evil") && !strings.Contains(err.Error(), "require") {
		t.Logf("error was: %v", err)
	}
}

func TestLuaPluginNoUnsafeGlobals(t *testing.T) {
	dir := luaPluginsDir(t)
	mgr := NewManager(hostnet.NewProxy(), dir)
	if err := mgr.Discover(); err != nil {
		t.Fatalf("Discover: %v", err)
	}
	defer func() { _ = mgr.Close() }()

	p := mgr.plugins["luatest"]
	L := p.lua
	if v := L.GetGlobal("io"); v.Type().String() != "nil" {
		t.Fatalf("io must be nil, got %v", v)
	}
	osTbl, ok := L.GetGlobal("os").(*lua.LTable)
	if !ok {
		t.Fatalf("os is not a table")
	}
	if v := osTbl.RawGetString("execute"); v.Type().String() != "nil" {
		t.Fatalf("os.execute must be nil")
	}
	if v := osTbl.RawGetString("time"); v.Type().String() == "nil" {
		t.Fatal("os.time must exist")
	}
}

func TestLuaPluginMissingGlobalsRejected(t *testing.T) {
	dir := t.TempDir()
	if err := copyDir("testdata/luatest", filepath.Join(dir, "luatest")); err != nil {
		t.Fatal(err)
	}
	// Drop one ABI global — load must fail.
	src := `PLUGIN = {contract_version = 1}
function search_manga(a) return "[]" end
function get_manga_detail(a) return "{}" end
function get_chapter_list(a) return "[]" end
`
	if err := writeFileForTest(filepath.Join(dir, "luatest", "main.lua"), src); err != nil {
		t.Fatal(err)
	}
	mgr := NewManager(hostnet.NewProxy(), dir)
	err := mgr.Discover()
	if err == nil {
		_ = mgr.Close()
		t.Fatal("Discover must fail when an ABI global is missing")
	}
	if !strings.Contains(err.Error(), "get_page_list") {
		t.Fatalf("error should name the missing global, got: %v", err)
	}
}

func TestLuaPluginInfiniteLoopTimesOut(t *testing.T) {
	dir := t.TempDir()
	if err := copyDir("testdata/luatest", filepath.Join(dir, "luatest")); err != nil {
		t.Fatal(err)
	}
	src := `PLUGIN = {contract_version = 1}
function search_manga(a) while true do end end
function get_manga_detail(a) return "{}" end
function get_chapter_list(a) return "[]" end
function get_page_list(a) return "[]" end
`
	if err := writeFileForTest(filepath.Join(dir, "luatest", "main.lua"), src); err != nil {
		t.Fatal(err)
	}
	mgr := NewManager(hostnet.NewProxy(), dir)
	if err := mgr.Discover(); err != nil {
		t.Fatalf("Discover: %v", err)
	}
	defer func() { _ = mgr.Close() }()

	done := make(chan error, 1)
	go func() {
		_, err := mgr.Search("luatest", types.SearchFilter{Query: "x"})
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("infinite loop must time out with an error")
		}
	case <-time.After(invokeTimeout + 15*time.Second):
		t.Fatal("search did not finish within invokeTimeout")
	}
}

func TestLuaPluginIDCollisionWithWasm(t *testing.T) {
	// A wasm file and lua folder with the same id must fail discovery.
	dir := luaPluginsDir(t)
	wasmPath := buildFixture(t, "plugin") // plugin.wasm
	if err := copyFile(wasmPath, filepath.Join(dir, "luatest.wasm")); err != nil {
		t.Fatal(err)
	}
	mgr := NewManager(hostnet.NewProxy(), dir)
	err := mgr.Discover()
	if err == nil {
		_ = mgr.Close()
		t.Fatal("Discover must fail on id collision")
	}
	if !strings.Contains(err.Error(), "collision") {
		t.Fatalf("error should mention collision, got: %v", err)
	}
}

func TestLuaPluginInstallFolder(t *testing.T) {
	// Install a lua folder from outside pluginsDir; hot-load must work.
	base := t.TempDir()
	pluginsDir := filepath.Join(base, "plugins")
	if err := copyDir("testdata/luatest", filepath.Join(base, "src", "luatest")); err != nil {
		t.Fatal(err)
	}
	mgr := NewManager(hostnet.NewProxy(), pluginsDir)
	if err := mgr.Discover(); err != nil {
		t.Fatalf("Discover: %v", err)
	}
	defer func() { _ = mgr.Close() }()

	path, err := mgr.Install(filepath.Join(base, "src", "luatest"))
	if err != nil {
		t.Fatalf("Install lua folder: %v", err)
	}
	if filepath.Base(path) != "main.lua" {
		t.Fatalf("Install should return main.lua path, got %s", path)
	}
	mangas, err := mgr.Search("luatest", types.SearchFilter{Query: "boom"})
	if err != nil {
		t.Fatalf("Search after install: %v", err)
	}
	if len(mangas) != 1 || mangas[0].Title != "Lua boom" {
		t.Fatalf("unexpected result: %+v", mangas)
	}
}

package pluginmanager

import (
	"path/filepath"
	"testing"

	"goisekai/internal/hostnet"
)

// ReloadPlugin must work for wasm too, whose wasmPath is the main.wasm FILE
// rather than a folder — the same shape a .so plugin has, and the reason the
// old reload (which passed wasmPath straight to the folder-based LoadPlugin)
// could load neither of them.
func TestReloadWasmPluginStaysUsable(t *testing.T) {
	dir := wasmFixtureDir(t)
	mgr := NewManager(hostnet.NewProxy(), dir)
	if err := mgr.Discover(); err != nil {
		t.Fatalf("Discover: %v", err)
	}
	defer func() { _ = mgr.Close() }()

	// This fixture's GetGenres is a static list, so the whole wasm path runs
	// without touching the network.
	if _, err := mgr.GetGenres("reimanga_net_cpp"); err != nil {
		t.Fatalf("GetGenres before reload: %v", err)
	}

	id, err := mgr.ReloadPlugin("reimanga_net_cpp")
	if err != nil {
		t.Fatalf("ReloadPlugin on a wasm plugin: %v", err)
	}
	if id != "reimanga_net_cpp" {
		t.Fatalf("reload returned id %q, want reimanga_net_cpp (the id must not change)", id)
	}

	genres, err := mgr.GetGenres("reimanga_net_cpp")
	if err != nil {
		t.Fatalf("GetGenres after reload: %v", err)
	}
	if len(genres) == 0 {
		t.Fatal("the reloaded wasm plugin returned no genres: the runtime was not re-instantiated")
	}
}

// wasmFixtureDir copies the smallest wasm plugin we ship (a C++ build, ~94 KB)
// into a temp plugins dir. Its GetGenres is a static list, so the whole wasm
// path — compile, instantiate, _initialize, contract_version, malloc/free and
// the JSON-over-linear-memory call — is exercised without any network.
func wasmFixtureDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	src := filepath.Join("..", "..", "examples", "plugins", "wasm", "reimanga_net_cpp")
	if err := copyDir(src, filepath.Join(dir, "reimanga_net_cpp")); err != nil {
		t.Fatalf("copy wasm fixture: %v", err)
	}
	return dir
}

// A wasm plugin registers from its folder at discovery, and loading it must
// read metadata from plugin.json like every other runtime — that is the part
// this change set moved out of the runtime.
func TestWasmPluginLoadsWithMetadataFromPluginJSON(t *testing.T) {
	dir := wasmFixtureDir(t)
	mgr := NewManager(hostnet.NewProxy(), dir)
	if err := mgr.Discover(); err != nil {
		t.Fatalf("Discover: %v", err)
	}
	defer func() { _ = mgr.Close() }()

	// Discovery is lazy for wasm too: no runtime yet.
	loaded := mgr.LoadedPlugins()
	if len(loaded) != 1 {
		t.Fatalf("registered %d plugin(s), want 1: %+v", len(loaded), loaded)
	}
	if got := loaded[0]; got.ID != "reimanga_net_cpp" || got.Kind != "wasm" || got.Loaded {
		t.Fatalf("unexpected registration: %+v", got)
	} else if got.Name != "ReiManga" || got.SiteURL != "https://reimanga.net" {
		t.Fatalf("metadata not read from plugin.json at discovery: %+v", got)
	}

	// GetGenres forces the lazy load and crosses the whole wasm ABI.
	genres, err := mgr.GetGenres("reimanga_net_cpp")
	if err != nil {
		t.Fatalf("GetGenres through wasm: %v", err)
	}
	if len(genres) == 0 {
		t.Fatal("wasm GetGenres returned nothing")
	}
	var sawAction bool
	for _, g := range genres {
		if g.Name == "Action" && g.Slug != "" {
			sawAction = true
		}
	}
	if !sawAction {
		t.Fatalf("unexpected genres from wasm: %+v", genres)
	}

	// After the invoke, metadata must still be the plugin.json values.
	after := mgr.LoadedPlugins()
	if len(after) != 1 || after[0].Name != "ReiManga" {
		t.Fatalf("metadata after load = %+v", after)
	}
	if after[0].Version != "1" {
		t.Fatalf("contract version = %q, want \"1\"", after[0].Version)
	}
}

// pack/unpack is the i64 return convention every wasm ABI call rides on:
// low 32 bits are the pointer, high 32 bits the length.
func TestWasmPackUnpackRoundTrip(t *testing.T) {
	for _, tc := range []struct{ ptr, length uint32 }{
		{0, 0},
		{16, 32},
		{0xffff, 1},
		{0x12345678, 0x9abcdef0},
	} {
		ptr, length := unpack(pack(tc.ptr, tc.length))
		if ptr != tc.ptr || length != tc.length {
			t.Errorf("pack(%d,%d) round-tripped to (%d,%d)", tc.ptr, tc.length, ptr, length)
		}
	}
}

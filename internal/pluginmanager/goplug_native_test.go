//go:build linux || darwin || freebsd || netbsd || openbsd

package pluginmanager

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"unsafe"

	"goisekai/internal/hostnet"
	"goisekai/pkg/types"
)

// nativeFixtureDir copies the dummy .so plugin into a temp plugins dir as a
// folder plugin (<dir>/dummy/main.so + plugin.json). The .so is gitignored and
// produced by examples/plugins/native/dummy/build.sh, so a checkout that never
// built it skips rather than fails.
func nativeFixtureDir(t *testing.T) string {
	t.Helper()
	src := filepath.Join("..", "..", "examples", "plugins", "native", "dummy")
	so := filepath.Join(src, "libdummy.so")
	if _, err := os.Stat(so); err != nil {
		t.Skipf("libdummy.so not built (run examples/plugins/native/dummy/build.sh): %v", err)
	}
	dir := t.TempDir()
	dst := filepath.Join(dir, "dummy")
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(so)
	if err != nil {
		t.Fatalf("read fixture .so: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dst, "main.so"), body, 0o755); err != nil {
		t.Fatal(err)
	}
	meta, err := os.ReadFile(filepath.Join(src, "plugin.json"))
	if err != nil {
		t.Fatalf("read fixture plugin.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dst, "plugin.json"), meta, 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// A .so plugin must register from its folder without being opened, carrying the
// metadata from plugin.json — the same lazy contract every other runtime keeps.
func TestNativePluginRegistersLazilyWithMetadata(t *testing.T) {
	dir := nativeFixtureDir(t)
	mgr := NewManager(hostnet.NewProxy(), dir)
	if err := mgr.Discover(); err != nil {
		t.Fatalf("Discover: %v", err)
	}
	defer func() { _ = mgr.Close() }()

	loaded := mgr.LoadedPlugins()
	if len(loaded) != 1 {
		t.Fatalf("registered %d plugin(s), want 1: %+v", len(loaded), loaded)
	}
	got := loaded[0]
	if got.ID != "dummy" || got.Kind != "go" {
		t.Fatalf("unexpected registration: %+v", got)
	}
	if got.Loaded {
		t.Fatal("plugin was instantiated at discovery; .so plugins must stay lazy")
	}
	// Metadata proves discoverGo read plugin.json beside the .so.
	if got.Name != "Dummy Native" || got.SiteURL != "https://example.invalid" {
		t.Fatalf("metadata not read from plugin.json: %+v", got)
	}
}

// The whole native ABI in one pass: dlopen, symbol binding, JSON in, JSON out,
// and plugin_free returning the buffer. The fixture echoes the query back, so a
// corrupted argument would show up as a wrong title.
func TestNativePluginCallCrossesABI(t *testing.T) {
	dir := nativeFixtureDir(t)
	mgr := NewManager(hostnet.NewProxy(), dir)
	if err := mgr.Discover(); err != nil {
		t.Fatalf("Discover: %v", err)
	}
	defer func() { _ = mgr.Close() }()

	results, err := mgr.Search("dummy", types.SearchFilter{Query: "harem"})
	if err != nil {
		t.Fatalf("Search through .so: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("got %d result(s), want 1: %+v", len(results), results)
	}
	// The fixture appends the query it parsed, so this asserts the input
	// string survived the Go→C boundary intact.
	if want := "dummy result:harem"; results[0].Title != want {
		t.Fatalf("title = %q, want %q", results[0].Title, want)
	}

	// GetGenres is the optional export; the fixture provides it.
	genres, err := mgr.GetGenres("dummy")
	if err != nil {
		t.Fatalf("GetGenres through .so: %v", err)
	}
	if len(genres) != 1 || genres[0].Name != "Action" {
		t.Fatalf("genres = %+v, want one Action", genres)
	}
	if _, err := mgr.GetMangaDetail("dummy", "dummy-1"); err != nil {
		t.Fatalf("GetMangaDetail through .so: %v", err)
	}
	if _, err := mgr.GetChapterList("dummy", "dummy-1"); err != nil {
		t.Fatalf("GetChapterList through .so: %v", err)
	}

	// GetPageList returns the error envelope, which must surface as an error
	// rather than an empty page list.
	if _, err := mgr.GetPageList("dummy", "c1"); err == nil {
		t.Fatal("GetPageList returned no error for the {\"error\"} envelope")
	}
}

// The host_call bridge: the fixture's GetMangaDetail asks the host to run
// text.titlecase and puts the answer in the description. A .so has no import
// table, so this only works if the host installed the plugin_init callback and
// the plugin could call back into Go through it.
func TestNativePluginReachesHostHelpers(t *testing.T) {
	dir := nativeFixtureDir(t)
	mgr := NewManager(hostnet.NewProxy(), dir)
	if err := mgr.Discover(); err != nil {
		t.Fatalf("Discover: %v", err)
	}
	defer func() { _ = mgr.Close() }()

	manga, err := mgr.GetMangaDetail("dummy", "dummy-1")
	if err != nil {
		t.Fatalf("GetMangaDetail through .so: %v", err)
	}
	// text.titlecase upper-cases the first rune: "host bridge" -> "Host bridge".
	if want := "Host bridge"; manga.Description != want {
		t.Fatalf("host helper result = %q, want %q (plugin_init bridge not installed?)",
			manga.Description, want)
	}
}

// ReloadPlugin must keep a native plugin usable. This is the regression that
// made reload route through UnloadPlugin+ensureLoaded: LoadPlugin has no
// main.so branch and treats a .so plugin's wasmPath — an entry FILE, not a
// folder — as a directory, so the old reload could never re-open it.
func TestReloadNativePluginStaysUsable(t *testing.T) {
	dir := nativeFixtureDir(t)
	mgr := NewManager(hostnet.NewProxy(), dir)
	if err := mgr.Discover(); err != nil {
		t.Fatalf("Discover: %v", err)
	}
	defer func() { _ = mgr.Close() }()

	// First call lazy-loads the .so, so reload has a live dlopen handle to
	// release. Skipping that dlclose would leave the old library mapped and
	// dlopen of the same path would hand the stale handle back.
	if _, err := mgr.Search("dummy", types.SearchFilter{Query: "before"}); err != nil {
		t.Fatalf("Search before reload: %v", err)
	}

	id, err := mgr.ReloadPlugin("dummy")
	if err != nil {
		t.Fatalf("ReloadPlugin on a native plugin: %v", err)
	}
	if id != "dummy" {
		t.Fatalf("reload returned id %q, want dummy (the id must not change)", id)
	}

	// The fixture echoes the query back, so this also proves the library
	// serving the call after reload is the plugin, not a stale mapping.
	results, err := mgr.Search("dummy", types.SearchFilter{Query: "after"})
	if err != nil {
		t.Fatalf("Search after reload: %v", err)
	}
	if want := "dummy result:after"; len(results) != 1 || results[0].Title != want {
		t.Fatalf("after reload: %+v, want one %q", results, want)
	}
}

// A reload that cannot load the library must leave the plugin registered, so
// fixing the file and reloading recovers it. The old implementation deleted the
// registry entry before loading, which is why a bad reload made a running
// plugin vanish from the UI until the process restarted.
//
// The broken plugin's .so is invalid from the start and is therefore never
// mmap'd: overwriting a .so that a running process already has mapped can
// SIGBUS the host (the write truncates pages the loader may still touch), so
// corrupting the loaded fixture is not a safe way to provoke this.
func TestFailedNativeReloadKeepsThePluginRegistered(t *testing.T) {
	dir := nativeFixtureDir(t)
	broken := filepath.Join(dir, "broken")
	if err := os.MkdirAll(broken, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(broken, "main.so"), []byte("not an ELF file"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(broken, "plugin.json"),
		[]byte(`{"contract_version":1}`), 0o644); err != nil {
		t.Fatal(err)
	}

	mgr := NewManager(hostnet.NewProxy(), dir)
	if err := mgr.Discover(); err != nil {
		t.Fatalf("Discover: %v", err)
	}
	defer func() { _ = mgr.Close() }()

	if _, err := mgr.ReloadPlugin("broken"); err == nil {
		t.Fatal("ReloadPlugin succeeded on an invalid .so")
	}
	if !slices.ContainsFunc(mgr.LoadedPlugins(), func(p LoadedPlugin) bool { return p.ID == "broken" }) {
		t.Fatal("a failed reload dropped the native plugin from the registry; it is unrecoverable until restart")
	}

	// A later call must report a load failure, not "plugin not registered":
	// the entry has to survive for the file to be fixable and reloaded.
	_, err := mgr.Search("broken", types.SearchFilter{Query: "x"})
	if err == nil {
		t.Fatal("Search on a plugin with an invalid .so succeeded")
	}
	if strings.Contains(err.Error(), "not registered") {
		t.Fatalf("the registry entry was lost: %v", err)
	}
}

// A native plugin must be installable and hot-loadable like every other kind.
// Install and LoadPlugin probed only main.lua/main.js/main.go/main.wasm, so a
// folder carrying main.so was rejected with "no main.lua, main.js, main.go, or
// main.wasm found" — a native plugin could be discovered but never installed.
func TestNativePluginInstallAndHotLoad(t *testing.T) {
	// nativeFixtureDir already produced a built folder plugin (<dir>/dummy).
	src := filepath.Join(nativeFixtureDir(t), "dummy")

	base := t.TempDir()
	mgr := NewManager(hostnet.NewProxy(), filepath.Join(base, "plugins"))
	if err := mgr.Discover(); err != nil {
		t.Fatalf("Discover: %v", err)
	}
	defer func() { _ = mgr.Close() }()

	path, err := mgr.Install(src)
	if err != nil {
		t.Fatalf("Install native folder: %v", err)
	}
	if filepath.Base(path) != "main.so" {
		t.Fatalf("Install should return the main.so path, got %s", path)
	}
	if _, err := mgr.Search("dummy", types.SearchFilter{Query: "installed"}); err != nil {
		t.Fatalf("Search after install: %v", err)
	}

	// Hot-load a second copy from an external path, which registers under the
	// folder name rather than the plugin's declared one.
	second := filepath.Join(base, "src", "dummy2")
	if err := copyDir(src, second); err != nil {
		t.Fatalf("stage second copy: %v", err)
	}
	id, err := mgr.LoadPlugin(second)
	if err != nil {
		t.Fatalf("LoadPlugin native folder: %v", err)
	}
	if id != "dummy2" {
		t.Fatalf("LoadPlugin returned %q, want dummy2", id)
	}
	if _, err := mgr.Search("dummy2", types.SearchFilter{Query: "hot"}); err != nil {
		t.Fatalf("Search after hot-load: %v", err)
	}
}

// cString walks a C string without purego's internal helper, so it gets its own
// check: a nil pointer and an embedded terminator both have to behave.
func TestCString(t *testing.T) {
	if got := cString(nil); got != "" {
		t.Fatalf("cString(nil) = %q, want empty", got)
	}
	buf := []byte("hello\x00world\x00")
	if got := cString(unsafe.Pointer(&buf[0])); got != "hello" {
		t.Fatalf("cString = %q, want %q", got, "hello")
	}
}

// Rewriting a loaded native plugin's .so in place — exactly what a plain `cp`
// or an installer's os.Create does — must not take the host down.
//
// This is the regression for the SIGSEGV that killed the server: dlopen keeps
// the file mmap'd, so an in-place O_TRUNC zeroes the same inode's pages, and
// the next call into the plugin (or the dlclose during ReloadPlugin) jumps
// into the hole. The fix loads the library from a private copy, leaving the
// on-disk file free to be rewritten.
//
// The failure mode is the process dying, so a regression shows up as a crashed
// test binary rather than a failed assertion — which a run of this test still
// catches.
func TestNativePluginSurvivesInPlaceOverwriteOfItsSo(t *testing.T) {
	dir := nativeFixtureDir(t)
	mgr := NewManager(hostnet.NewProxy(), dir)
	if err := mgr.Discover(); err != nil {
		t.Fatalf("Discover: %v", err)
	}
	defer func() { _ = mgr.Close() }()

	// Load it once, so the library is mapped for the rest of the test.
	if _, err := mgr.Search("dummy", types.SearchFilter{Query: "before"}); err != nil {
		t.Fatalf("Search before overwrite: %v", err)
	}

	soPath := filepath.Join(dir, "dummy", "main.so")
	body, err := os.ReadFile(soPath)
	if err != nil {
		t.Fatalf("read %s: %v", soPath, err)
	}
	before, err := os.Stat(soPath)
	if err != nil {
		t.Fatalf("stat %s: %v", soPath, err)
	}
	if err := os.WriteFile(soPath, body, 0o755); err != nil {
		t.Fatalf("overwrite in place: %v", err)
	}
	after, err := os.Stat(soPath)
	if err != nil {
		t.Fatalf("restat %s: %v", soPath, err)
	}
	// os.WriteFile opens with O_TRUNC on the same inode, which is the whole
	// point: if it ever stopped doing that, this test would prove nothing.
	if !os.SameFile(before, after) {
		t.Fatalf("overwrite replaced the inode; test no longer reproduces the bug")
	}

	// The mapped library must still be callable...
	if _, err := mgr.Search("dummy", types.SearchFilter{Query: "after"}); err != nil {
		t.Fatalf("Search after in-place overwrite: %v", err)
	}
	// ...and a reload (which dlcloses and reloads) must not crash either.
	if _, err := mgr.ReloadPlugin("dummy"); err != nil {
		t.Fatalf("ReloadPlugin after in-place overwrite: %v", err)
	}
	results, err := mgr.Search("dummy", types.SearchFilter{Query: "reloaded"})
	if err != nil {
		t.Fatalf("Search after reload: %v", err)
	}
	if want := "dummy result:reloaded"; len(results) != 1 || results[0].Title != want {
		t.Fatalf("after reload: %+v, want one %q", results, want)
	}
}

//go:build linux || darwin || freebsd || netbsd || openbsd

package pluginmanager

import (
	"os"
	"path/filepath"
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

	// The fixture returns the error envelope from GetPageList, which must
	// surface as an error rather than an empty page list.
	if _, err := mgr.GetPageList("dummy", "c1"); err == nil {
		t.Fatal("GetPageList returned no error for the {\"error\"} envelope")
	}
}

// cString walks a C string without purego's internal helper, so it gets its own
// check: a nil pointer and an embedded terminator both have to behave.
func TestCString(t *testing.T) {
	if got := cString(0); got != "" {
		t.Fatalf("cString(0) = %q, want empty", got)
	}
	buf := []byte("hello\x00world\x00")
	if got := cString(uintptr(unsafe.Pointer(&buf[0]))); got != "hello" {
		t.Fatalf("cString = %q, want %q", got, "hello")
	}
}

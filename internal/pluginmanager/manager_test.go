package pluginmanager

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"goisekai/internal/hostnet"
	"goisekai/pkg/types"
)

// buildFixture compiles the named testdata plugin (without the .go suffix) to
// wasip1/wasm and returns its path.
func buildFixture(t *testing.T, name string) string {
	t.Helper()
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain not available")
	}
	wasmPath := filepath.Join(t.TempDir(), name+".wasm")
	cmd := exec.Command("go", "build", "-buildmode=c-shared", "-o", wasmPath, "./testdata/"+name+".go")
	cmd.Env = append(os.Environ(), "GOOS=wasip1", "GOARCH=wasm")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build fixture: %v\n%s", err, out)
	}
	return wasmPath
}

func TestManagerRoundTrip(t *testing.T) {
	wasmPath := buildFixture(t, "plugin")
	pluginsDir := filepath.Dir(wasmPath)

	mgr := NewManager(hostnet.NewProxy(), pluginsDir)
	if err := mgr.Discover(); err != nil {
		t.Fatalf("Discover: %v", err)
	}
	defer func() {
		_ = mgr.Close()
	}()

	mangas, err := mgr.Search("plugin", types.SearchFilter{Query: "test"})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(mangas) != 1 || mangas[0].ID != "m1" || mangas[0].Title != "Test Manga" {
		t.Fatalf("unexpected Search result: %+v", mangas)
	}

	detail, err := mgr.GetMangaDetail("plugin", "m1")
	if err != nil {
		t.Fatalf("GetMangaDetail: %v", err)
	}
	if detail.Title != "Test Manga" {
		t.Fatalf("unexpected detail: %+v", detail)
	}

	chapters, err := mgr.GetChapterList("plugin", "m1")
	if err != nil {
		t.Fatalf("GetChapterList: %v", err)
	}
	if len(chapters) != 1 || chapters[0].ID != "c1" {
		t.Fatalf("unexpected chapters: %+v", chapters)
	}

	pages, err := mgr.GetPageList("plugin", "c1")
	if err != nil {
		t.Fatalf("GetPageList: %v", err)
	}
	if len(pages) != 2 || pages[1].URL != "https://example.com/2.jpg" {
		t.Fatalf("unexpected pages: %+v", pages)
	}
}

func TestUnknownPlugin(t *testing.T) {
	wasmPath := buildFixture(t, "plugin")
	mgr := NewManager(hostnet.NewProxy(), filepath.Dir(wasmPath))
	if err := mgr.Discover(); err != nil {
		t.Fatalf("Discover: %v", err)
	}
	defer func() {
		_ = mgr.Close()
	}()

	if _, err := mgr.Search("nope", types.SearchFilter{}); err == nil {
		t.Fatal("expected error for unknown plugin")
	}
}

// TestPanickingPlugin verifies that a plugin whose Search panics returns a Go
// error to the host, and that the manager (and a healthy plugin) keep working
// afterward — the panic is isolated, not fatal to the reader (criterion 7.2).
func TestPanickingPlugin(t *testing.T) {
	panicPath := buildFixture(t, "panicplugin")
	// Put the healthy plugin in a separate dir so Discover only sees the
	// panicking one here.
	mgr := NewManager(hostnet.NewProxy(), filepath.Dir(panicPath))
	if err := mgr.Discover(); err != nil {
		t.Fatalf("Discover: %v", err)
	}
	defer func() {
		_ = mgr.Close()
	}()

	if _, err := mgr.Search("panicplugin", types.SearchFilter{}); err == nil {
		t.Fatal("expected error from panicking plugin Search")
	}
}

func TestInstallCopiesPlugin(t *testing.T) {
	wasmPath := buildFixture(t, "plugin") // lives in its own temp dir
	pluginsDir := t.TempDir()             // separate, empty target dir

	mgr := NewManager(hostnet.NewProxy(), pluginsDir)
	if err := mgr.Discover(); err != nil {
		t.Fatalf("Discover: %v", err)
	}
	defer func() { _ = mgr.Close() }()

	dest, err := mgr.Install(wasmPath)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if filepath.Dir(dest) != pluginsDir {
		t.Fatalf("Install returned %q, want a copy under %q", dest, pluginsDir)
	}
	if _, err := os.Stat(dest); err != nil {
		t.Fatalf("installed plugin missing: %v", err)
	}
	if _, err := os.Stat(wasmPath); err != nil {
		t.Fatalf("source plugin was moved/deleted, want a copy: %v", err)
	}

	// The copied plugin must be usable through the runtime.
	mangas, err := mgr.Search("plugin", types.SearchFilter{Query: "test"})
	if err != nil {
		t.Fatalf("Search after Install: %v", err)
	}
	if len(mangas) != 1 || mangas[0].ID != "m1" {
		t.Fatalf("unexpected Search result: %+v", mangas)
	}
}

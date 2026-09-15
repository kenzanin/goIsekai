package pluginmanager

import (
	"path/filepath"
	"testing"

	"goisekai/internal/enrich"
	"goisekai/internal/hostnet"
)

// infoTestManager builds a Manager with the fixture info script installed and
// returns it with the enrichment registry it registered into. The plugins dir
// is empty, so anything the catalog reports came from the info script.
func infoTestManager(t *testing.T) (*Manager, *enrich.Registry) {
	t.Helper()
	infoDir := t.TempDir()
	if err := copyDir("testdata/info", infoDir); err != nil {
		t.Fatalf("copy info fixture: %v", err)
	}
	mgr := NewManager(hostnet.NewProxy(), t.TempDir())
	mgr.SetInfoDir(infoDir)
	reg := enrich.NewRegistry()
	mgr.SetEnrichRegistry(reg)
	if err := mgr.Discover(); err != nil {
		t.Fatalf("Discover: %v", err)
	}
	return mgr, reg
}

// An info script implements only getEnrichment, so discovery must accept it
// without any of the source ABI functions.
func TestInfoScriptLoadsWithoutSourceABI(t *testing.T) {
	mgr, _ := infoTestManager(t)
	defer func() { _ = mgr.Close() }()

	mgr.LoadEnrichmentProviders()

	p, err := mgr.get(infoPrefix + "testsource")
	if err != nil {
		t.Fatalf("info script not registered: %v", err)
	}
	if !p.loaded {
		t.Fatal("info script did not load")
	}
}

// The declared provider must reach the enrichment catalog, keyed by the bare
// folder name rather than the namespaced manager id.
func TestInfoScriptRegistersEnrichmentProvider(t *testing.T) {
	mgr, reg := infoTestManager(t)
	defer func() { _ = mgr.Close() }()

	mgr.LoadEnrichmentProviders()

	entry := reg.Resolve("testsource")
	if entry == nil {
		t.Fatal("provider testsource not registered")
	}
	if entry.Name() != "Test Source" {
		t.Fatalf("unexpected provider name %q", entry.Name())
	}
	if !reg.SupportsKind("testsource", enrich.KindAuthors) {
		t.Fatal("expected testsource to support the authors kind")
	}
}

// An info script is metadata-only: it must never show up as a manga source.
func TestInfoScriptIsNotAMangaSource(t *testing.T) {
	mgr, _ := infoTestManager(t)
	defer func() { _ = mgr.Close() }()

	mgr.LoadEnrichmentProviders()

	for _, p := range mgr.LoadedPlugins() {
		if p.ID == infoPrefix+"testsource" || p.ID == "testsource" {
			t.Fatalf("info script leaked into the plugin list as %q", p.ID)
		}
	}
}

// getEnrichment must round-trip through the ABI: request JSON in, item array
// out, with the source stamped by the host.
func TestInfoScriptGetEnrichment(t *testing.T) {
	mgr, _ := infoTestManager(t)
	defer func() { _ = mgr.Close() }()

	mgr.LoadEnrichmentProviders()

	items, err := mgr.GetEnrichment(infoPrefix+"testsource", "Some Manga", string(enrich.KindAuthors), "testsource")
	if err != nil {
		t.Fatalf("GetEnrichment: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Value != "Author A" {
		t.Fatalf("unexpected item value %q", items[0].Value)
	}
	if items[0].URL != "https://example.test/a" {
		t.Fatalf("unexpected item url %q", items[0].URL)
	}
	if items[0].Source != "testsource" {
		t.Fatalf("expected host to stamp the source, got %q", items[0].Source)
	}
}

// An unsupported kind is not an error the user should see: it comes back empty.
func TestInfoScriptUnknownKindIsEmpty(t *testing.T) {
	mgr, _ := infoTestManager(t)
	defer func() { _ = mgr.Close() }()

	mgr.LoadEnrichmentProviders()

	items, err := mgr.GetEnrichment(infoPrefix+"testsource", "Some Manga", "nonsense", "testsource")
	if err != nil {
		t.Fatalf("GetEnrichment: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected no items, got %d", len(items))
	}
}

// The shipped MangaDex info script must parse and declare the five field kinds
// the detail page stores.
func TestShippedMangaDexInfoScriptDeclaresKinds(t *testing.T) {
	infoDir := t.TempDir()
	if err := copyDir(filepath.Join("..", "..", "examples", "info"), infoDir); err != nil {
		t.Fatalf("copy shipped info scripts: %v", err)
	}
	mgr := NewManager(hostnet.NewProxy(), t.TempDir())
	mgr.SetInfoDir(infoDir)
	reg := enrich.NewRegistry()
	mgr.SetEnrichRegistry(reg)
	if err := mgr.Discover(); err != nil {
		t.Fatalf("Discover: %v", err)
	}
	defer func() { _ = mgr.Close() }()

	mgr.LoadEnrichmentProviders()

	if reg.Resolve("mangadex") == nil {
		t.Fatal("shipped script did not register a mangadex provider")
	}
	for _, kind := range []enrich.Kind{
		enrich.KindTitles,
		enrich.KindSummaries,
		enrich.KindCategories,
		enrich.KindAuthors,
		enrich.KindRelated,
	} {
		if !reg.SupportsKind("mangadex", kind) {
			t.Fatalf("mangadex provider does not declare kind %q", kind)
		}
	}
}

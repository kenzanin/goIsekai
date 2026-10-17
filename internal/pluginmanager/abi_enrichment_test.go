package pluginmanager

import (
	"os"
	"path/filepath"
	"testing"

	"goisekai/internal/hostnet"
	"goisekai/pkg/types"
)

// The enrichment entry point belongs to enrichment scripts, not to source
// plugins. Splitting the maps is what makes that structural: a source plugin is
// verified against the source ABI only, so it can never be asked for an
// enrichment export it was never meant to have.
func TestSourceABIDoesNotIncludeEnrichment(t *testing.T) {
	t.Parallel()

	if _, ok := luaFnNames[types.GetEnrichmentFunc]; ok {
		t.Error("luaFnNames must not carry the enrichment entry point")
	}
	if _, ok := jsFnNames[types.GetEnrichmentFunc]; ok {
		t.Error("jsFnNames must not carry the enrichment entry point")
	}
	if _, ok := luaInfoFnNames[types.GetEnrichmentFunc]; !ok {
		t.Error("luaInfoFnNames must carry the enrichment entry point")
	}
}

// A source plugin that exports only the source ABI still loads: absence of an
// enrichment export is not a defect.
func TestSourcePluginLoadsWithoutEnrichmentExport(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writePluginFolder(t, dir, "noenrich", "main.lua", `
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

	if _, err := mgr.GetMangaDetail("noenrich", "m1"); err != nil {
		t.Fatalf("a source plugin without an enrichment export must still load: %v", err)
	}
}

// An enrichment script declares its provider and answers get_enrichment. The
// canonical name is snake_case, matching the host name like every other runtime.
func TestInfoScriptUsesCanonicalEnrichmentName(t *testing.T) {
	t.Parallel()

	infoDir := t.TempDir()
	src := `
function get_enrichment(a) return '[{"value":"canonical","url":"https://example.test/1"}]' end
`
	if err := os.MkdirAll(filepath.Join(infoDir, "canon"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(infoDir, "canon", "main.lua"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	meta := `{"contract_version":1,"enrichment_providers":[{"id":"canon","name":"Canon","kinds":["titles"]}]}`
	if err := os.WriteFile(filepath.Join(infoDir, "canon", "plugin.json"), []byte(meta), 0o644); err != nil {
		t.Fatal(err)
	}

	mgr := NewManager(hostnet.NewProxy(), t.TempDir())
	mgr.SetInfoDir(infoDir)
	if err := mgr.Discover(); err != nil {
		t.Fatalf("Discover: %v", err)
	}
	defer func() { _ = mgr.Close() }()

	items, err := mgr.GetEnrichment(infoPrefix+"canon", "Some Manga", "titles", "canon")
	if err != nil {
		t.Fatalf("GetEnrichment: %v", err)
	}
	if len(items) != 1 || items[0].Value != "canonical" {
		t.Fatalf("unexpected items: %+v", items)
	}
}

// The pre-normalisation camelCase name keeps working for scripts already out
// there, with a deprecation warning rather than a load failure.
func TestInfoScriptStillAcceptsLegacyEnrichmentName(t *testing.T) {
	t.Parallel()

	infoDir := t.TempDir()
	src := `
function getEnrichment(a) return '[{"value":"legacy","url":"https://example.test/2"}]' end
`
	if err := os.MkdirAll(filepath.Join(infoDir, "legacy"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(infoDir, "legacy", "main.lua"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	meta := `{"contract_version":1,"enrichment_providers":[{"id":"legacy","name":"Legacy","kinds":["titles"]}]}`
	if err := os.WriteFile(filepath.Join(infoDir, "legacy", "plugin.json"), []byte(meta), 0o644); err != nil {
		t.Fatal(err)
	}

	mgr := NewManager(hostnet.NewProxy(), t.TempDir())
	mgr.SetInfoDir(infoDir)
	if err := mgr.Discover(); err != nil {
		t.Fatalf("Discover: %v", err)
	}
	defer func() { _ = mgr.Close() }()

	items, err := mgr.GetEnrichment(infoPrefix+"legacy", "Some Manga", "titles", "legacy")
	if err != nil {
		t.Fatalf("GetEnrichment via legacy name: %v", err)
	}
	if len(items) != 1 || items[0].Value != "legacy" {
		t.Fatalf("unexpected items: %+v", items)
	}
}

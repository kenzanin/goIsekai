package pluginmanager

import (
	"errors"
	"testing"

	"goisekai/internal/hostnet"
	"goisekai/pkg/types"
)

// engineFor must return a usable engine for every kind the host ships, and a
// named error for one that does not exist. This is what lets a new runtime be
// added by registering a single entry instead of editing every dispatch site.
func TestEngineForResolvesEveryShippedKind(t *testing.T) {
	t.Parallel()

	for kind := range engines {
		e, err := engineFor(kind)
		if err != nil {
			t.Fatalf("engineFor(%q) error = %v", kind, err)
		}
		if e == nil {
			t.Fatalf("engineFor(%q) returned nil engine", kind)
		}
	}

	const want = 5
	if len(engines) != want {
		t.Fatalf("len(engines) = %d, want %d", len(engines), want)
	}
	for _, kind := range []string{"lua", "js", "native", "yaegi", "wasm"} {
		if _, ok := engines[kind]; !ok {
			t.Fatalf("engines missing shipped kind %q", kind)
		}
	}
}

// An unregistered kind must surface a wrapped sentinel error, not panic.
func TestEngineForUnknownKindErrors(t *testing.T) {
	t.Parallel()

	_, err := engineFor("ruby")
	if err == nil {
		t.Fatal("engineFor(unknown) must error")
	}
	if !errors.Is(err, ErrUnsupportedKind) {
		t.Fatalf("engineFor(unknown) error = %v, want wrap of ErrUnsupportedKind", err)
	}
	if got := err.Error(); got != `unsupported plugin kind "ruby"` {
		t.Fatalf("engineFor(unknown) message = %q", got)
	}
}

// The full load -> call -> close path for a lua and a js plugin, driven only
// through the registry, proves the interface is wired end to end.
func TestRegistryDrivesLoadCallClose(t *testing.T) {
	t.Parallel()

	codeByKind := map[string]struct {
		id    string
		entry string
		code  string
	}{
		"lua": {"reglua", "main.lua", `
function search(a) return '[{"id":"R1","title":"registry lua"}]' end
function get_manga_detail(a) return '{"id":"R1"}' end
function get_chapter_list(a) return '[]' end
function get_page_list(a) return '[]' end
`},
		"js": {"regjs", "main.js", `
function search(a) { return JSON.stringify([{id:"R1",title:"registry js"}]); }
function getMangaDetail(a) { return JSON.stringify({id:"R1"}); }
function getChapterList(a) { return "[]"; }
function getPageList(a) { return "[]"; }
`},
	}

	for kind, fx := range codeByKind {
		t.Run(kind, func(t *testing.T) {
			dir := t.TempDir()
			writePluginFolder(t, dir, fx.id, fx.entry, fx.code)

			mgr := NewManager(hostnet.NewProxy(), dir)
			if err := mgr.Discover(); err != nil {
				t.Fatalf("Discover: %v", err)
			}
			defer func() { _ = mgr.Close() }()

			// load + call, all through the public surface which routes via engineFor.
			mangas, err := mgr.Search(fx.id, types.SearchFilter{Query: "x"})
			if err != nil {
				t.Fatalf("Search via registry: %v", err)
			}
			if len(mangas) != 1 || mangas[0].ID != "R1" {
				t.Fatalf("unexpected result: %+v", mangas)
			}

			// close path: unload must release the runtime through the registry.
			if err := mgr.UnloadPlugin(fx.id); err != nil {
				t.Fatalf("UnloadPlugin via registry: %v", err)
			}
			// re-load on the next call, still through the registry.
			mangas, err = mgr.Search(fx.id, types.SearchFilter{Query: "x"})
			if err != nil {
				t.Fatalf("Search after unload: %v", err)
			}
			if len(mangas) != 1 || mangas[0].ID != "R1" {
				t.Fatalf("unexpected result after re-load: %+v", mangas)
			}
		})
	}
}

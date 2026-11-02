package pluginmanager

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"goisekai/internal/hostnet"
	"goisekai/pkg/types"
)

// registryFixture builds the lightest shipped plugin for one kind. It returns
// the path engine.load receives (a folder for lua, js and yaegi; the entry file
// for go and wasm, as ensureLoaded passes wasmPath) and the ABI function to
// call. A builder that cannot run here skips only its own subtest.
type registryFixture func(t *testing.T) (path, fn string)

// registryFixtures has one entry per registered kind. The walk below reads it
// as data and never branches on the kind itself.
var registryFixtures = map[string]registryFixture{
	"lua": func(t *testing.T) (string, string) {
		return filepath.Join("testdata", "luatest"), types.SearchFunc
	},
	"js": func(t *testing.T) (string, string) {
		return filepath.Join("testdata", "jstest"), types.SearchFunc
	},
	"yaegi": func(t *testing.T) (string, string) {
		return filepath.Join("testdata", "yaegitest"), types.SearchFunc
	},
	"native": func(t *testing.T) (string, string) {
		dir := nativeFixtureDir(t)
		return filepath.Join(dir, "dummy", "main.so"), types.SearchFunc
	},
	"wasm": func(t *testing.T) (string, string) {
		dir := wasmFixtureDir(t)
		// GetGenres is a static list, so no network is touched.
		return filepath.Join(dir, "reimanga_net_cpp", "main.wasm"), types.GetGenresFunc
	},
}

// Every kind in the registry must load, call and close through the engine
// interface alone. Adding a kind without a fixture fails here on purpose.
func TestEngineRegistryWalksEveryKind(t *testing.T) {
	for kind := range engines {
		if _, ok := registryFixtures[kind]; !ok {
			t.Fatalf("registered kind %q has no registry fixture", kind)
		}
	}

	for kind := range engines {
		t.Run(kind, func(t *testing.T) {
			e, err := engineFor(kind)
			if err != nil {
				t.Fatalf("engineFor(%q) error = %v", kind, err)
			}
			if e == nil {
				t.Fatalf("engineFor(%q) returned nil engine", kind)
			}

			path, fn := registryFixtures[kind](t)
			m := NewManager(hostnet.NewProxy(), t.TempDir())
			defer func() { _ = m.Close() }()

			p, err := e.load(m, "reg_"+kind, path)
			if err != nil {
				t.Fatalf("load(%q) error = %v", kind, err)
			}
			if p == nil {
				t.Fatalf("load(%q) returned nil plugin", kind)
			}

			out, err := e.call(m, p, fn, `{"query":"x"}`)
			if err != nil {
				t.Fatalf("call(%q, %s) error = %v", kind, fn, err)
			}
			if !json.Valid([]byte(out)) {
				t.Fatalf("call(%q, %s) returned non-JSON %q", kind, fn, out)
			}

			if err := e.close(m, p); err != nil {
				t.Fatalf("close(%q) error = %v", kind, err)
			}
		})
	}
}

// A kind with no registered runtime must return the named sentinel, both from
// the registry lookup and from the lazy-load path that dispatches through it,
// and must not panic.
func TestEngineRegistryUnregisteredKindErrors(t *testing.T) {
	for _, kind := range []string{"ruby", ""} {
		e, err := engineFor(kind)
		if e != nil {
			t.Fatalf("engineFor(%q) returned engine for unregistered kind", kind)
		}
		if !errors.Is(err, ErrUnsupportedKind) {
			t.Fatalf("engineFor(%q) error = %v, want wrap of ErrUnsupportedKind", kind, err)
		}
	}

	m := NewManager(hostnet.NewProxy(), t.TempDir())
	m.plugins["ghost"] = &loadedPlugin{id: "ghost", kind: "ruby"}
	err := m.ensureLoaded("ghost")
	if !errors.Is(err, ErrUnsupportedKind) {
		t.Fatalf("ensureLoaded(ghost) error = %v, want wrap of ErrUnsupportedKind", err)
	}
	if !strings.Contains(err.Error(), `"ruby"`) {
		t.Fatalf("ensureLoaded(ghost) error = %q, want it to name kind ruby", err)
	}
}

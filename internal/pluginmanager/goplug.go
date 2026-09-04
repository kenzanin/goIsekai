package pluginmanager

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"plugin"
	"strings"

	"goisekai/pkg/types"
)

// Go native plugin support (pkg.go.dev/plugin). Preparation for future
// plugin types: a compiled .so exposing the ABI as exported symbols with
// the same JSON-in/JSON-out contract as JS/Lua/WASM plugins.
//
// Each symbol has signature: func(string) (string, error)
// Expected exported symbols: Search, GetMangaDetail, GetChapterList, GetPageList
// plus an exported `Meta` variable of type map[string]any (or a
// ContractVersion int32 + PluginMeta types.PluginMeta struct).
//
// ponytail: the host binary builds with CGO_ENABLED=0, and Go's plugin
// package requires cgo + matching toolchain at runtime. Loading fails with
// a clear error today; when a real .so plugin arrives, build the host with
// CGO_ENABLED=1 and identical Go version/flags as the .so.

// goFnNames maps host ABI function names to the PascalCase exported symbols
// a Go native plugin provides.
var goFnNames = map[string]string{
	types.SearchFunc:         "Search",
	types.GetMangaDetailFunc: "GetMangaDetail",
	types.GetChapterListFunc: "GetChapterList",
	types.GetPageListFunc:    "GetPageList",
}

// loadGo opens a compiled Go plugin (.so) and validates its ABI symbols.
func (m *Manager) loadGo(id, path string) (*loadedPlugin, error) {
	h, err := plugin.Open(path)
	if err != nil {
		return nil, fmt.Errorf("go plugin open: %w", err)
	}

	// Contract version must be an exported int32 variable named
	// ContractVersion.
	sym, err := h.Lookup("ContractVersion")
	if err != nil {
		return nil, fmt.Errorf("go plugin %s: exported var ContractVersion missing: %w", id, err)
	}
	verPtr, ok := sym.(*int32)
	if !ok {
		return nil, fmt.Errorf("go plugin %s: ContractVersion must be int32", id)
	}
	contractVer := *verPtr
	if err := types.CheckVersion(contractVer); err != nil {
		return nil, fmt.Errorf("go plugin %s: %w", id, err)
	}

	// Meta must be an exported types.PluginMeta variable named Meta.
	metaSym, err := h.Lookup("Meta")
	if err != nil {
		return nil, fmt.Errorf("go plugin %s: exported var Meta missing: %w", id, err)
	}
	metaPtr, ok := metaSym.(*types.PluginMeta)
	if !ok {
		return nil, fmt.Errorf("go plugin %s: Meta must be types.PluginMeta", id)
	}

	// Verify every ABI symbol resolves to a func(string) (string, error).
	fns := map[string]any{}
	for hostName, symName := range goFnNames {
		s, err := h.Lookup(symName)
		if err != nil {
			return nil, fmt.Errorf("go plugin %s: symbol %s missing: %w", id, symName, err)
		}
		fn, ok := s.(func(string) (string, error))
		if !ok {
			return nil, fmt.Errorf("go plugin %s: symbol %s must be func(string) (string, error)", id, symName)
		}
		fns[hostName] = fn
	}

	return &loadedPlugin{
		id:              id,
		wasmPath:        path,
		kind:            "go",
		loaded:          true,
		contractVersion: contractVer,
		meta:            *metaPtr,
		goPlugin:        h,
		goFns:           fns,
	}, nil
}

// callGo invokes an ABI function on a Go native plugin.
func callGo(p *loadedPlugin, fnName, inputJSON string) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	sym, ok := p.goFns[fnName]
	if !ok {
		return "", fmt.Errorf("go plugin %s: no symbol for %s", p.id, fnName)
	}
	fn, ok := sym.(func(string) (string, error))
	if !ok {
		return "", fmt.Errorf("go plugin %s %s: bad symbol type", p.id, fnName)
	}
	out, err := fn(inputJSON)
	if err != nil {
		return "", fmt.Errorf("go plugin %s %s: %w", p.id, fnName, err)
	}
	if strings.TrimSpace(out) == "" {
		return "", fmt.Errorf("go plugin %s %s: empty result", p.id, fnName)
	}
	var probe any
	if err := json.Unmarshal([]byte(out), &probe); err != nil {
		return "", fmt.Errorf("go plugin %s %s: result is not JSON: %w", p.id, fnName, err)
	}
	return out, nil
}

// discoverGo registers *.so files in the plugins dir as native Go plugins.
func (m *Manager) discoverGo() error {
	matches, err := filepath.Glob(filepath.Join(m.pluginsDir, "*.so"))
	if err != nil {
		return err
	}
	for _, path := range matches {
		id := strings.TrimSuffix(filepath.Base(path), ".so")
		if _, dup := m.plugins[id]; dup {
			continue
		}
		m.plugins[id] = &loadedPlugin{
			id:       id,
			wasmPath: path,
			kind:     "go",
		}
	}
	return nil
}

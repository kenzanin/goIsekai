package pluginmanager

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"

	"goisekai/internal/logger"
	"goisekai/pkg/types"
)

const (
	// memoryLimitPages caps a plugin's linear memory at 64 MB (1024 * 64 KiB).
	memoryLimitPages = 1024
	// hostModuleName is the module name plugins import host functions from.
	hostModuleName = "env"
)

// wasmRequiredFns are the ABI entry points a wasm SOURCE plugin must export.
// An enrichment script (infoOnly) exports GetEnrichment instead — requiring
// the scraper ABI from it would reject every info script.
var wasmRequiredFns = []string{
	types.SearchFunc,
	types.GetMangaDetailFunc,
	types.GetChapterListFunc,
	types.GetPageListFunc,
}

// wasmInfoRequiredFns are the entry points a wasm enrichment script must export.
var wasmInfoRequiredFns = []string{types.GetEnrichmentFunc}

// loadWasm compiles, instantiates and version-checks a single wasm plugin.
func (m *Manager) loadWasm(id, path string) (*loadedPlugin, error) {
	rt, err := m.ensureWasmRuntime()
	if err != nil {
		return nil, fmt.Errorf("wasm plugin %s: %w", id, err)
	}
	// The plugin's own params must be readable BEFORE its code runs: a wasm
	// plugin calls host_call from _initialize, and the host answers with the
	// params its own plugin.json declared.
	m.loadParams(id, filepath.Dir(path))
	code, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("wasm plugin %s: %w", id, err)
	}
	compiled, err := rt.CompileModule(m.ctx, code)
	if err != nil {
		return nil, fmt.Errorf("wasm plugin %s: compile: %w", id, err)
	}
	// WithStartFunctions() clears the default _start so a reactor module is not
	// run to completion at instantiation; _initialize is called explicitly below.
	//
	// WithSysWalltime is NOT cosmetic: wazero defaults WASI's realtime clock to
	// platform.NewFakeWalltime(), which starts at 2022-01-01 and advances 1ms per
	// read. A plugin that reads the clock (mangataro signs its chapter request
	// with a timestamp) therefore produced a token the site had long expired, and
	// the host saw a bare 403 with no hint that the clock was the problem.
	mod, err := rt.InstantiateModule(m.ctx, compiled, wazero.NewModuleConfig().
		WithName(id).
		WithStartFunctions().
		WithSysWalltime())
	if err != nil {
		return nil, fmt.Errorf("wasm plugin %s: instantiate: %w", id, err)
	}
	// Reactor entry: Go/TinyGo wasip1 modules built with -buildmode=c-shared
	// export _initialize instead of _start. Run it once to reach a ready state.
	if initFn := mod.ExportedFunction("_initialize"); initFn != nil {
		if _, err := initFn.Call(m.ctx); err != nil {
			return nil, fmt.Errorf("wasm plugin %s: _initialize: %w", id, err)
		}
	}

	verFn := mod.ExportedFunction(types.ContractVersionFunc)
	if verFn == nil {
		return nil, fmt.Errorf("wasm plugin %s: does not export %s", id, types.ContractVersionFunc)
	}
	verRes, err := verFn.Call(m.ctx)
	if err != nil {
		return nil, fmt.Errorf("wasm plugin %s: contract_version: %w", id, err)
	}
	if len(verRes) == 0 {
		return nil, fmt.Errorf("wasm plugin %s: contract_version returned no result", id)
	}
	ver := int32(verRes[0])
	if err := types.CheckVersion(ver); err != nil {
		return nil, fmt.Errorf("wasm plugin %s: %w", id, err)
	}

	// An enrichment script exports the metadata ABI, not the scraper ABI; the
	// id prefix is what distinguishes it (infoPrefix), so no extra plumbing
	// has to cross the three load call sites.
	requiredFns := wasmRequiredFns
	if isInfoPlugin(id) {
		requiredFns = wasmInfoRequiredFns
	}
	fns := make(map[string]api.Function, len(requiredFns)+2)
	for _, name := range requiredFns {
		fn := mod.ExportedFunction(name)
		if fn == nil {
			return nil, fmt.Errorf("wasm plugin %s: does not export %s", id, name)
		}
		fns[name] = fn
	}
	// GetGenres and GetEnrichment are optional: a source plugin may omit the
	// former, and an info script is only ever asked for the latter.
	for _, name := range []string{types.GetGenresFunc, types.GetEnrichmentFunc} {
		if fn := mod.ExportedFunction(name); fn != nil {
			fns[name] = fn
		}
	}

	logger.Debug("wasm plugin loaded", "id", id, "path", path)
	meta, err := pluginMetaFor(id, "wasm", filepath.Dir(path))
	if err != nil {
		return nil, err
	}
	return &loadedPlugin{
		id:              id,
		wasmPath:        path,
		kind:            "wasm",
		loaded:          true,
		wasmMod:         mod,
		wasmFns:         fns,
		contractVersion: ver,
		meta:            meta,
	}, nil
}

package pluginmanager

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"

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

// ensureWasmRuntime lazily creates the shared wazero runtime once,
// instantiating WASI and the host module ("env" exporting host_http_request and
// the generic host_call that reaches every Lua/JS host helper).
//
// It takes runtimeMu, NOT mu: the callers (Install, LoadPlugin) already hold
// mu, and sync.RWMutex is not reentrant, so taking mu again here would
// self-deadlock the host on every wasm plugin install.
func (m *Manager) ensureWasmRuntime() (wazero.Runtime, error) {
	m.runtimeMu.Lock()
	defer m.runtimeMu.Unlock()
	if m.runtime != nil {
		return m.runtime, nil
	}
	rt := wazero.NewRuntimeWithConfig(m.ctx,
		wazero.NewRuntimeConfig().WithMemoryLimitPages(memoryLimitPages).WithCloseOnContextDone(true))
	if _, err := wasi_snapshot_preview1.Instantiate(m.ctx, rt); err != nil {
		_ = rt.Close(m.ctx)
		return nil, fmt.Errorf("instantiate wasi: %w", err)
	}
	if _, err := rt.NewHostModuleBuilder(hostModuleName).
		NewFunctionBuilder().
		WithGoModuleFunction(api.GoModuleFunc(m.hostHTTPRequest),
			[]api.ValueType{api.ValueTypeI32, api.ValueTypeI32},
			[]api.ValueType{api.ValueTypeI64}).
		Export(types.HostHTTPRequestFunc).
		NewFunctionBuilder().
		WithGoModuleFunction(api.GoModuleFunc(m.hostCall),
			[]api.ValueType{api.ValueTypeI32, api.ValueTypeI32},
			[]api.ValueType{api.ValueTypeI64}).
		Export(types.HostCallFunc).
		Instantiate(m.ctx); err != nil {
		_ = rt.Close(m.ctx)
		return nil, fmt.Errorf("instantiate host module: %w", err)
	}
	m.runtime = rt
	return rt, nil
}

// loadWasm compiles, instantiates and version-checks a single wasm plugin.
func (m *Manager) loadWasm(id, path string) (*loadedPlugin, error) {
	rt, err := m.ensureWasmRuntime()
	if err != nil {
		return nil, fmt.Errorf("wasm plugin %s: %w", id, err)
	}
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

// wasmInvokeResult finalises a wasm invocation. A challenge parked by an HTTP
// helper during the call wins over whatever the guest returned: the guest could
// only see a nil/error body and would otherwise report an empty result, hiding
// the fact that the site wants a human to verify. Returning the typed error
// here is the wasm counterpart of the Lua natives aborting the call.
func (m *Manager) wasmInvokeResult(id, out string) (string, error) {
	if err := m.takeChallenge(id); err != nil {
		return "", err
	}
	return out, nil
}

func (m *Manager) callWasm(p *loadedPlugin, fnName, inputJSON string) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Challenges are per-invocation: drop anything an earlier call left behind
	// so a past block cannot fail this one.
	m.clearChallenge(p.id)

	if p.wasmMod == nil {
		return "", fmt.Errorf("wasm plugin %s: not loaded", p.id)
	}
	fn, ok := p.wasmFns[fnName]
	if !ok {
		return "", fmt.Errorf("wasm plugin %s: no export %s", p.id, fnName)
	}

	ctx, cancel := context.WithTimeout(context.Background(), p.invokeDeadline())
	defer cancel()

	input := []byte(inputJSON)
	inPtr, ok := wasmAlloc(ctx, p.wasmMod, uint32(len(input)))
	if !ok {
		return "", fmt.Errorf("wasm plugin %s %s: malloc failed for input", p.id, fnName)
	}
	defer wasmFree(ctx, p.wasmMod, inPtr)
	if !p.wasmMod.Memory().Write(inPtr, input) {
		return "", fmt.Errorf("wasm plugin %s %s: write input out of range", p.id, fnName)
	}

	results, err := fn.Call(ctx, uint64(inPtr), uint64(len(input)))
	if err != nil {
		return "", fmt.Errorf("wasm plugin %s %s: %w", p.id, fnName, err)
	}
	if len(results) == 0 {
		return "", fmt.Errorf("wasm plugin %s %s: no result", p.id, fnName)
	}
	outPtr, outLen := unpack(results[0])
	if outPtr == 0 || outLen == 0 {
		return "", fmt.Errorf("wasm plugin %s %s: empty result", p.id, fnName)
	}
	defer wasmFree(ctx, p.wasmMod, outPtr)
	out, ok := p.wasmMod.Memory().Read(outPtr, outLen)
	if !ok {
		return "", fmt.Errorf("wasm plugin %s %s: result out of range", p.id, fnName)
	}
	return m.wasmInvokeResult(p.id, string(out))
}

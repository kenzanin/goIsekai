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

// wasmRequiredFns are the ABI entry points every wasm plugin must export.
var wasmRequiredFns = []string{
	types.SearchFunc,
	types.GetMangaDetailFunc,
	types.GetChapterListFunc,
	types.GetPageListFunc,
}

// ensureWasmRuntime lazily creates the shared wazero runtime once,
// instantiating WASI and the host module ("env" exporting host_http_request and
// the generic host_call that reaches every Lua/JS host helper).
func (m *Manager) ensureWasmRuntime() (wazero.Runtime, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
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
	mod, err := rt.InstantiateModule(m.ctx, compiled, wazero.NewModuleConfig().WithName(id).WithStartFunctions())
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

	fns := make(map[string]api.Function, len(wasmRequiredFns)+1)
	for _, name := range wasmRequiredFns {
		fn := mod.ExportedFunction(name)
		if fn == nil {
			return nil, fmt.Errorf("wasm plugin %s: does not export %s", id, name)
		}
		fns[name] = fn
	}
	// GetGenres is optional.
	if fn := mod.ExportedFunction(types.GetGenresFunc); fn != nil {
		fns[types.GetGenresFunc] = fn
	}

	logger.Info("wasm plugin loaded", "id", id, "path", path)
	return &loadedPlugin{
		id:              id,
		wasmPath:        path,
		kind:            "wasm",
		loaded:          true,
		wasmMod:         mod,
		wasmFns:         fns,
		contractVersion: ver,
		meta:            readPluginJSONMeta(filepath.Dir(path)),
	}, nil
}

// callWasm invokes one JSON-in/JSON-out ABI function over wasm linear memory.
func callWasm(p *loadedPlugin, fnName, inputJSON string) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

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
	return string(out), nil
}

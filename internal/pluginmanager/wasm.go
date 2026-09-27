package pluginmanager

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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

	ctx, cancel := context.WithTimeout(context.Background(), invokeTimeout)
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

// hostHTTPRequest is the env.host_http_request host import. The guest passes
// (ptr, len) of an HTTPRequest JSON and receives a packed i64 (ptr|len<<32)
// response buffer, allocated from the guest's own malloc.
func (m *Manager) hostHTTPRequest(ctx context.Context, mod api.Module, stack []uint64) {
	ptr, length := uint32(stack[0]), uint32(stack[1])
	reqBytes, ok := mod.Memory().Read(ptr, length)
	if !ok {
		stack[0] = pack(0, 0)
		return
	}
	respJSON, err := m.proxy.HandleRequest(mod.Name(), string(reqBytes))
	if err != nil {
		logger.Warn("wasm host_http_request failed", "plugin", mod.Name(), "error", err)
		stack[0] = pack(0, 0)
		return
	}
	resp := []byte(respJSON)
	respPtr, ok := wasmAlloc(ctx, mod, uint32(len(resp)))
	if !ok || !mod.Memory().Write(respPtr, resp) {
		stack[0] = pack(0, 0)
		return
	}
	stack[0] = pack(respPtr, uint32(len(resp)))
}

// wasmAlloc allocates size bytes in the module's linear memory via its
// exported malloc, or ok=false when the module lacks one or allocation fails.
func wasmAlloc(ctx context.Context, mod api.Module, size uint32) (uint32, bool) {
	malloc := mod.ExportedFunction("malloc")
	if malloc == nil {
		return 0, false
	}
	res, err := malloc.Call(ctx, uint64(size))
	if err != nil || len(res) == 0 {
		return 0, false
	}
	ptr := uint32(res[0])
	if ptr == 0 {
		return 0, false
	}
	return ptr, true
}

// wasmFree releases a buffer by calling the module's exported free, if any.
func wasmFree(ctx context.Context, mod api.Module, ptr uint32) {
	if free := mod.ExportedFunction("free"); free != nil {
		_, _ = free.Call(ctx, uint64(ptr))
	}
}

// pack combines pointer and length into one i64 ABI return
// (low 32 bits = pointer, high 32 bits = length).
func pack(ptr, length uint32) uint64 { return uint64(length)<<32 | uint64(ptr) }

// unpack splits a packed i64 back into (pointer, length).
func unpack(v uint64) (ptr, length uint32) { return uint32(v), uint32(v >> 32) }

// discoverWasm registers wasm-kind plugins without instantiating them:
// <pluginsDir>/*/main.wasm (folder plugin) and <pluginsDir>/*.wasm (single file).
func (m *Manager) discoverWasm() error {
	globs := []struct {
		pattern string
		folder  bool
	}{
		{filepath.Join(m.pluginsDir, "*", "main.wasm"), true},
		{filepath.Join(m.pluginsDir, "*.wasm"), false},
	}
	for _, g := range globs {
		matches, err := filepath.Glob(g.pattern)
		if err != nil {
			return err
		}
		for _, path := range matches {
			id := strings.TrimSuffix(filepath.Base(path), ".wasm")
			if g.folder {
				id = filepath.Base(filepath.Dir(path))
			}
			if _, dup := m.plugins[id]; dup {
				logger.Error("plugin id collision, skipping", "id", id, "kind", "wasm")
				continue
			}
			lp := &loadedPlugin{id: id, wasmPath: path, kind: "wasm"}
			if g.folder {
				// Seed site_url before first load so gated image CDNs get a
				// Referer without a plugin invoke (lazy plugins stay lazy).
				lp.meta = readPluginJSONMeta(filepath.Dir(path))
			}
			m.plugins[id] = lp
			logger.Info("wasm plugin registered", "id", id, "path", path)
		}
	}
	return nil
}

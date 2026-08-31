package pluginmanager

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"

	"goisekai/internal/logger"
	"goisekai/pkg/types"
)

// load compiles, instantiates, and version-checks a single plugin wasm file.
func (m *Manager) load(rt wazero.Runtime, id, wasmPath string) (*loadedPlugin, error) {
	code, err := os.ReadFile(wasmPath)
	if err != nil {
		return nil, err
	}
	compiled, err := rt.CompileModule(m.ctx, code)
	if err != nil {
		return nil, fmt.Errorf("compile: %w", err)
	}
	mod, err := rt.InstantiateModule(m.ctx, compiled, wazero.NewModuleConfig().WithName(id).WithStartFunctions())
	if err != nil {
		return nil, fmt.Errorf("instantiate: %w", err)
	}

	// A Go wasip1 plugin built with -buildmode=c-shared exports _initialize (the
	// reactor entry) instead of _start. Run it once to bring the runtime to a
	// resident, ready state; unlike _start it does not run main() or exit.
	if initFn := mod.ExportedFunction("_initialize"); initFn != nil {
		if _, err := initFn.Call(m.ctx); err != nil {
			return nil, fmt.Errorf("_initialize: %w", err)
		}
	}

	verFn := mod.ExportedFunction(types.ContractVersionFunc)
	if verFn == nil {
		return nil, fmt.Errorf("does not export %s", types.ContractVersionFunc)
	}
	verRes, err := verFn.Call(m.ctx)
	if err != nil {
		return nil, fmt.Errorf("contract_version: %w", err)
	}
	if len(verRes) == 0 {
		return nil, fmt.Errorf("contract_version returned no result")
	}
	if err := types.CheckVersion(int32(verRes[0])); err != nil {
		return nil, err
	}
	logger.Debug("contract_version resolved", "id", id, "version", int32(verRes[0]))

	p := &loadedPlugin{id: id, wasmPath: wasmPath, mod: mod, contractVersion: int32(verRes[0]), fn: make(map[string]api.Function)}
	for _, name := range []string{types.SearchFunc, types.GetMangaDetailFunc, types.GetChapterListFunc, types.GetPageListFunc} {
		f := mod.ExportedFunction(name)
		if f == nil {
			return nil, fmt.Errorf("does not export %s", name)
		}
		p.fn[name] = f
	}
	p.meta = m.readInit(p)
	return p, nil
}

// alloc returns a pointer to size bytes in the plugin's linear memory by
// calling its exported malloc, or ok=false when the plugin lacks one or the
// allocation fails.
func (m *Manager) alloc(p *loadedPlugin, size uint32) (uint32, bool) {
	malloc := p.mod.ExportedFunction("malloc")
	if malloc == nil {
		return 0, false
	}
	res, err := malloc.Call(m.ctx, uint64(size))
	if err != nil || len(res) == 0 {
		return 0, false
	}
	ptr := uint32(res[0])
	if ptr == 0 {
		return 0, false
	}
	return ptr, true
}

// free releases a plugin-malloc'd buffer via the plugin's exported free, when
// present. Best-effort: a plugin without free simply leaks.
func (m *Manager) free(p *loadedPlugin, ptr uint32) {
	if ptr == 0 {
		return
	}
	f := p.mod.ExportedFunction("free")
	if f == nil {
		return
	}
	_, _ = f.Call(m.ctx, uint64(ptr))
}

// hostHTTPRequest implements env.host_http_request(ptr, len) -> packed(ptr, len).
// It reads the request JSON from the plugin's memory, delegates to the hostnet
// proxy (keyed by plugin id for cookie isolation), and writes the response JSON
// back into a plugin-malloc'd buffer.
func (m *Manager) hostHTTPRequest(ctx context.Context, mod api.Module, ptr, length uint32) uint64 {
	reqBytes, ok := mod.Memory().Read(ptr, length)
	if !ok {
		return pack(0, 0)
	}
	respJSON, err := m.proxy.HandleRequest(mod.Name(), string(reqBytes))
	if err != nil {
		// Surface the failure to the plugin as an empty response rather than
		// trapping; plugins can detect empty bodies.
		return pack(0, 0)
	}
	resp := []byte(respJSON)

	malloc := mod.ExportedFunction("malloc")
	if malloc == nil {
		return pack(0, 0)
	}
	allocRes, err := malloc.Call(ctx, uint64(len(resp)))
	if err != nil || len(allocRes) == 0 {
		return pack(0, 0)
	}
	respPtr := uint32(allocRes[0])
	if respPtr == 0 || !mod.Memory().Write(respPtr, resp) {
		return pack(0, 0)
	}
	return pack(respPtr, uint32(len(resp)))
}

// pack combines a pointer and length into the single i64 the ABI returns
// (low 32 bits = pointer, high 32 bits = length).
func pack(ptr, length uint32) uint64 {
	return uint64(length)<<32 | uint64(ptr)
}

// readInit optionally invokes a plugin's Init export to collect its declared
// metadata (verify_url, needs_human_verify, thumb_ratio). Init is optional
// (D7: no ABI version bump), so an absent or failing Init is not fatal: the
// plugin simply contributes zero metadata.
func (m *Manager) readInit(p *loadedPlugin) types.PluginMeta {
	initFn := p.mod.ExportedFunction(types.InitFunc)
	if initFn == nil {
		return types.PluginMeta{}
	}
	ctx, cancel := context.WithTimeout(m.ctx, invokeTimeout)
	defer cancel()
	results, err := initFn.Call(ctx)
	if err != nil {
		logger.Debug("plugin Init failed", "id", p.id, "error", err)
		return types.PluginMeta{}
	}
	if len(results) == 0 {
		return types.PluginMeta{}
	}
	outPtr, outLen := unpack(results[0])
	if outPtr == 0 || outLen == 0 {
		return types.PluginMeta{}
	}
	defer m.free(p, outPtr)
	out, ok := p.mod.Memory().Read(outPtr, outLen)
	if !ok {
		return types.PluginMeta{}
	}
	var meta types.PluginMeta
	if err := json.Unmarshal(out, &meta); err != nil {
		logger.Debug("plugin Init returned invalid JSON", "id", p.id, "error", err)
		return types.PluginMeta{}
	}
	return meta
}

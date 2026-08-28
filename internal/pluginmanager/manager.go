package pluginmanager

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"

	"goisekai/internal/hostnet"
	"goisekai/pkg/types"
)

const (
	// memoryLimitPages caps a plugin's linear memory at 64 MB (1024 * 64 KiB).
	memoryLimitPages = 1024
	// invokeTimeout bounds a single plugin invocation (5.3: 15 s).
	invokeTimeout = 15 * time.Second
	// hostModuleName is the module name plugins import host functions from.
	hostModuleName = "env"
)

// loadedPlugin is a compiled and instantiated WASM plugin and its resolved ABI
// entry points.
type loadedPlugin struct {
	id       string
	wasmPath string
	mod      api.Module
	// fn maps an ABI function name (types.*Func) to its resolved api.Function.
	fn map[string]api.Function
	// mu serializes invocations: api.Function.Call is not goroutine-safe, and a
	// single plugin instance must not be re-entered concurrently.
	mu sync.Mutex
}

// Manager loads WASM source plugins and exposes their search/detail operations
// to the host. It owns the shared wazero runtime and wires host_http_request to
// the hostnet proxy.
type Manager struct {
	proxy      *hostnet.Proxy
	pluginsDir string
	ctx        context.Context

	mu      sync.RWMutex
	runtime wazero.Runtime
	plugins map[string]*loadedPlugin
}

// NewManager returns a Manager that will load plugins from pluginsDir and route
// their network access through proxy.
func NewManager(proxy *hostnet.Proxy, pluginsDir string) *Manager {
	return &Manager{
		proxy:      proxy,
		pluginsDir: pluginsDir,
		ctx:        context.Background(),
		plugins:    make(map[string]*loadedPlugin),
	}
}

// Discover creates the wazero runtime, instantiates the host module, and loads
// every *.wasm file in pluginsDir. A plugin that fails its contract-version
// check or instantiation aborts the whole discovery with a descriptive error.
func (m *Manager) Discover() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	rt := wazero.NewRuntimeWithConfig(m.ctx,
		wazero.NewRuntimeConfig().
			WithMemoryLimitPages(memoryLimitPages).
			WithCloseOnContextDone(true))

	// Go and TinyGo wasip1 modules import WASI; instantiate it so plugin
	// runtimes that need it can start (5.1: wasip1 target support).
	if _, err := wasi_snapshot_preview1.Instantiate(m.ctx, rt); err != nil {
		return fmt.Errorf("instantiate wasi_snapshot_preview1: %w", err)
	}

	if _, err := rt.NewHostModuleBuilder(hostModuleName).
		NewFunctionBuilder().
		WithFunc(m.hostHTTPRequest).
		Export(types.HostHTTPRequestFunc).
		Instantiate(m.ctx); err != nil {
		return fmt.Errorf("instantiate host module: %w", err)
	}

	matches, err := filepath.Glob(filepath.Join(m.pluginsDir, "*.wasm"))
	if err != nil {
		return err
	}

	m.runtime = rt
	for _, path := range matches {
		id := strings.TrimSuffix(filepath.Base(path), ".wasm")
		p, err := m.load(rt, id, path)
		if err != nil {
			_ = rt.Close(m.ctx)
			return fmt.Errorf("load plugin %s: %w", id, err)
		}
		m.plugins[id] = p
	}
	return nil
}

// Close releases the runtime and all instantiated plugins.
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.runtime == nil {
		return nil
	}
	return m.runtime.Close(m.ctx)
}

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

	p := &loadedPlugin{id: id, wasmPath: wasmPath, mod: mod, fn: make(map[string]api.Function)}
	for _, name := range []string{types.SearchFunc, types.GetMangaDetailFunc, types.GetChapterListFunc, types.GetPageListFunc} {
		f := mod.ExportedFunction(name)
		if f == nil {
			return nil, fmt.Errorf("does not export %s", name)
		}
		p.fn[name] = f
	}
	return p, nil
}

// get returns the loaded plugin for pluginID under a read lock.
func (m *Manager) get(pluginID string) (*loadedPlugin, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	p, ok := m.plugins[pluginID]
	if !ok {
		return nil, fmt.Errorf("plugin %q not loaded", pluginID)
	}
	return p, nil
}

// call invokes one JSON-in/JSON-out ABI function on a plugin, enforcing the
// per-invocation timeout. A panic or trap inside the plugin surfaces as an
// error here rather than crashing the host.
func (m *Manager) call(p *loadedPlugin, fnName, inputJSON string) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	ctx, cancel := context.WithTimeout(m.ctx, invokeTimeout)
	defer cancel()

	input := []byte(inputJSON)
	inPtr, ok := m.alloc(p, uint32(len(input)))
	if !ok {
		return "", fmt.Errorf("plugin %s: malloc failed for input", p.id)
	}
	defer m.free(p, inPtr)
	if !p.mod.Memory().Write(inPtr, input) {
		return "", fmt.Errorf("plugin %s: write input out of range", p.id)
	}

	results, err := p.fn[fnName].Call(ctx, uint64(inPtr), uint64(len(input)))
	if err != nil {
		return "", fmt.Errorf("plugin %s %s: %w", p.id, fnName, err)
	}
	if len(results) == 0 {
		return "", fmt.Errorf("plugin %s %s: no result", p.id, fnName)
	}
	outPtr, outLen := unpack(results[0])
	if outPtr == 0 || outLen == 0 {
		return "", fmt.Errorf("plugin %s %s: empty result", p.id, fnName)
	}
	defer m.free(p, outPtr)
	out, ok := p.mod.Memory().Read(outPtr, outLen)
	if !ok {
		return "", fmt.Errorf("plugin %s %s: result out of range", p.id, fnName)
	}
	return string(out), nil
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

// unpack splits the packed i64 back into (pointer, length).
func unpack(v uint64) (ptr, length uint32) {
	return uint32(v & 0xffffffff), uint32(v >> 32)
}

// Search runs a plugin's Search function and decodes its result.
func (m *Manager) Search(pluginID string, filter types.SearchFilter) ([]types.Manga, error) {
	p, err := m.get(pluginID)
	if err != nil {
		return nil, err
	}
	in, err := json.Marshal(filter)
	if err != nil {
		return nil, err
	}
	out, err := m.call(p, types.SearchFunc, string(in))
	if err != nil {
		return nil, err
	}
	var result []types.Manga
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		return nil, fmt.Errorf("plugin %s: invalid Search result: %w", pluginID, err)
	}
	return result, nil
}

// GetMangaDetail runs a plugin's GetMangaDetail function and decodes its result.
func (m *Manager) GetMangaDetail(pluginID, mangaID string) (types.Manga, error) {
	p, err := m.get(pluginID)
	if err != nil {
		return types.Manga{}, err
	}
	in, err := json.Marshal(mangaID)
	if err != nil {
		return types.Manga{}, err
	}
	out, err := m.call(p, types.GetMangaDetailFunc, string(in))
	if err != nil {
		return types.Manga{}, err
	}
	var result types.Manga
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		return types.Manga{}, fmt.Errorf("plugin %s: invalid GetMangaDetail result: %w", pluginID, err)
	}
	return result, nil
}

// GetChapterList runs a plugin's GetChapterList function and decodes its result.
func (m *Manager) GetChapterList(pluginID, mangaID string) ([]types.Chapter, error) {
	p, err := m.get(pluginID)
	if err != nil {
		return nil, err
	}
	in, err := json.Marshal(mangaID)
	if err != nil {
		return nil, err
	}
	out, err := m.call(p, types.GetChapterListFunc, string(in))
	if err != nil {
		return nil, err
	}
	var result []types.Chapter
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		return nil, fmt.Errorf("plugin %s: invalid GetChapterList result: %w", pluginID, err)
	}
	return result, nil
}

// GetPageList runs a plugin's GetPageList function and decodes its result.
func (m *Manager) GetPageList(pluginID, chapterID string) ([]types.Page, error) {
	p, err := m.get(pluginID)
	if err != nil {
		return nil, err
	}
	in, err := json.Marshal(chapterID)
	if err != nil {
		return nil, err
	}
	out, err := m.call(p, types.GetPageListFunc, string(in))
	if err != nil {
		return nil, err
	}
	var result []types.Page
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		return nil, fmt.Errorf("plugin %s: invalid GetPageList result: %w", pluginID, err)
	}
	return result, nil
}

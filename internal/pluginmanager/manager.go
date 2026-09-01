package pluginmanager

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
	lua "github.com/yuin/gopher-lua"

	"goisekai/internal/hostnet"
	"goisekai/internal/logger"
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

// loadedPlugin is a compiled and instantiated plugin and its resolved ABI
// entry points. kind is "wasm" or "lua"; only the relevant fields are set.
type loadedPlugin struct {
	id       string
	wasmPath string
	kind     string // "wasm" or "lua"
	mod      api.Module
	// fn maps an ABI function name (types.*Func) to its resolved api.Function.
	fn map[string]api.Function
	// lua holds the Lua VM for lua-kind plugins (nil for wasm).
	lua *lua.LState
	// contractVersion is the plugin's resolved contract_version.
	contractVersion int32
	// meta is the metadata the plugin declared in its optional Init export.
	meta types.PluginMeta
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

	pattern := filepath.Join(m.pluginsDir, "*.wasm")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return err
	}
	logger.Debug("discovering plugins", "dir", m.pluginsDir, "count", len(matches))

	m.runtime = rt
	for _, path := range matches {
		id := strings.TrimSuffix(filepath.Base(path), ".wasm")
		logger.Debug("loading plugin", "id", id, "path", path)
		p, err := m.load(rt, id, path)
		if err != nil {
			logger.Error("plugin load failed", "id", id, "error", err)
			_ = rt.Close(m.ctx)
			return fmt.Errorf("load plugin %s: %w", id, err)
		}
		m.plugins[id] = p
		m.proxy.SetNeedsJS(id, p.meta.NeedsJS)
		logger.Debug("plugin loaded", "id", id, "version", p.contractVersion)
	}

	// Lua plugins: one folder per plugin, main.lua entry, folder name = id.
	luaMatches, err := filepath.Glob(filepath.Join(m.pluginsDir, "*", "main.lua"))
	if err != nil {
		return err
	}
	for _, path := range luaMatches {
		id := filepath.Base(filepath.Dir(path))
		if _, dup := m.plugins[id]; dup {
			return fmt.Errorf("plugin id collision: %q (wasm and lua)", id)
		}
		logger.Debug("loading lua plugin", "id", id, "path", path)
		p, err := m.loadLua(id, filepath.Dir(path))
		if err != nil {
			logger.Error("lua plugin load failed", "id", id, "error", err)
			return fmt.Errorf("load lua plugin %s: %w", id, err)
		}
		m.plugins[id] = p
		m.proxy.SetNeedsJS(id, p.meta.NeedsJS)
		logger.Debug("lua plugin loaded", "id", id, "version", p.contractVersion)
	}
	return nil
}

// Install copies a plugin file (wasm) or folder (lua, containing main.lua)
// into pluginsDir, hot-loads it, and registers it under its base name. It
// must be called after Discover. It returns the path of the copy inside
// pluginsDir, which the caller should persist as the plugin's WasmPath.
func (m *Manager) Install(wasmPath string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.runtime == nil {
		return "", fmt.Errorf("Discover must be called before Install")
	}
	id := strings.TrimSuffix(filepath.Base(wasmPath), ".wasm")

	// Lua plugin: source is a folder containing main.lua; copy it recursively.
	mainLua := filepath.Join(wasmPath, "main.lua")
	if info, err := os.Stat(mainLua); err == nil && !info.IsDir() {
		id = filepath.Base(wasmPath)
		destDir := filepath.Join(m.pluginsDir, id)
		logger.Debug("installing lua plugin", "source", wasmPath, "dest", destDir)
		if filepath.Clean(wasmPath) != filepath.Clean(destDir) {
			if err := copyDir(wasmPath, destDir); err != nil {
				return "", fmt.Errorf("copy lua plugin %s: %w", id, err)
			}
		}
		p, err := m.loadLua(id, destDir)
		if err != nil {
			logger.Error("lua plugin install failed", "id", id, "error", err)
			return "", fmt.Errorf("install lua plugin %s: %w", id, err)
		}
		m.plugins[id] = p
		m.proxy.SetNeedsJS(id, p.meta.NeedsJS)
		logger.Debug("lua plugin installed", "id", id)
		return filepath.Join(destDir, "main.lua"), nil
	}

	dest := filepath.Join(m.pluginsDir, id+".wasm")
	logger.Debug("installing plugin", "source", wasmPath, "dest", dest)
	if filepath.Clean(wasmPath) != filepath.Clean(dest) {
		if err := copyFile(wasmPath, dest); err != nil {
			return "", fmt.Errorf("copy plugin %s: %w", id, err)
		}
	}
	p, err := m.load(m.runtime, id, dest)
	if err != nil {
		logger.Error("plugin install failed", "id", id, "error", err)
		return "", fmt.Errorf("install plugin %s: %w", id, err)
	}
	m.plugins[id] = p
	m.proxy.SetNeedsJS(id, p.meta.NeedsJS)
	logger.Debug("plugin installed", "id", id)
	return dest, nil
}

// copyDir recursively copies src dir to dst (lua plugin folders).
func copyDir(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	for _, e := range entries {
		s := filepath.Join(src, e.Name())
		d := filepath.Join(dst, e.Name())
		if e.IsDir() {
			if err := copyDir(s, d); err != nil {
				return err
			}
			continue
		}
		if err := copyFile(s, d); err != nil {
			return err
		}
	}
	return nil
}

// Close releases the runtime and all instantiated plugins.
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, p := range m.plugins {
		if p.kind == "lua" && p.lua != nil {
			p.lua.Close()
		}
	}
	if m.runtime == nil {
		return nil
	}
	return m.runtime.Close(m.ctx)
}

// copyFile copies src to dst, truncating dst if it exists.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

// LoadedPlugin is metadata about a currently-loaded plugin.
type LoadedPlugin struct {
	ID               string
	Version          string // ABI contract version (e.g. "1")
	WasmPath         string
	VerifyURL        string // from the plugin's optional Init metadata
	NeedsHumanVerify bool
	ThumbRatio       float64
	NeedsJS          bool
}

// LoadedPlugins returns metadata for every plugin currently loaded in memory,
// sorted by id.
func (m *Manager) LoadedPlugins() []LoadedPlugin {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]LoadedPlugin, 0, len(m.plugins))
	for _, p := range m.plugins {
		out = append(out, LoadedPlugin{
			ID:               p.id,
			Version:          strconv.Itoa(int(p.contractVersion)),
			WasmPath:         p.wasmPath,
			VerifyURL:        p.meta.VerifyURL,
			NeedsHumanVerify: p.meta.NeedsHumanVerify,
			ThumbRatio:       p.meta.ThumbRatio,
			NeedsJS:          p.meta.NeedsJS,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

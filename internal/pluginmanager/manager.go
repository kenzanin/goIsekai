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

	"github.com/dop251/goja"
	"github.com/extism/go-sdk"

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
	// hostModuleName is no longer used after Extism migration.
	hostModuleName = "env" // ponytail: remove when no other legacy code uses this const

)

// loadedPlugin is a compiled and instantiated plugin and its resolved ABI
// entry points. kind is "wasm" or "lua"; only the relevant fields are set.
type loadedPlugin struct {
	id       string
	wasmPath string
	kind     string // "wasm", "lua", or "js"
	// extismPlugin holds the Extism plugin instance for wasm-kind plugins (nil otherwise).
	extismPlugin *extism.Plugin
	// lua holds the Lua VM for lua-kind plugins (nil for wasm/js).
	lua *lua.LState
	// js holds the goja VM for js-kind plugins (nil otherwise).
	js *goja.Runtime
	// contractVersion is the plugin's resolved contract_version.
	contractVersion int32
	// meta is the metadata the plugin declared in its optional Init export.
	meta types.PluginMeta
	// mu serializes invocations: concurrent calls to the same plugin must not interleave.
	mu sync.Mutex
}

// Manager loads WASM source plugins and exposes their search/detail operations
// to the host. It wires host_http_request to
// the hostnet proxy.
type Manager struct {
	proxy      *hostnet.Proxy
	pluginsDir string
	ctx        context.Context

	mu      sync.RWMutex
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

// Discover loads
// every *.wasm file in pluginsDir. A plugin that fails its contract-version
// check or instantiation aborts the whole discovery with a descriptive error.
func (m *Manager) Discover() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// WASM plugins: one .wasm file per plugin, filename (minus .wasm) = id.
	pattern := filepath.Join(m.pluginsDir, "*.wasm")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return err
	}
	logger.Debug("discovering plugins", "dir", m.pluginsDir, "count", len(matches))

	for _, path := range matches {
		id := strings.TrimSuffix(filepath.Base(path), ".wasm")
		logger.Debug("loading plugin", "id", id, "path", path)
		p, err := m.load(id, path)
		if err != nil {
			logger.Error("plugin load failed", "id", id, "error", err)
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
	// JS plugins: one folder per plugin, main.js entry, folder name = id.
	jsMatches, err := filepath.Glob(filepath.Join(m.pluginsDir, "*", "main.js"))
	if err != nil {
		return err
	}
	for _, path := range jsMatches {
		id := filepath.Base(filepath.Dir(path))
		if _, dup := m.plugins[id]; dup {
			return fmt.Errorf("plugin id collision: %q (wasm/lua and js)", id)
		}
		logger.Debug("loading js plugin", "id", id, "path", path)
		p, err := m.loadJS(id, filepath.Dir(path))
		if err != nil {
			logger.Error("js plugin load failed", "id", id, "error", err)
			return fmt.Errorf("load js plugin %s: %w", id, err)
		}
		m.plugins[id] = p
		m.proxy.SetNeedsJS(id, p.meta.NeedsJS)
		logger.Debug("js plugin loaded", "id", id, "version", p.contractVersion)
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
	if m.plugins == nil {
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
	// JS plugin: source is a folder containing main.js; copy it recursively.
	mainJS := filepath.Join(wasmPath, "main.js")
	if info, err := os.Stat(mainJS); err == nil && !info.IsDir() {
		id = filepath.Base(wasmPath)
		destDir := filepath.Join(m.pluginsDir, id)
		logger.Debug("installing js plugin", "source", wasmPath, "dest", destDir)
		if filepath.Clean(wasmPath) != filepath.Clean(destDir) {
			if err := copyDir(wasmPath, destDir); err != nil {
				return "", fmt.Errorf("copy js plugin %s: %w", id, err)
			}
		}
		p, err := m.loadJS(id, destDir)
		if err != nil {
			logger.Error("js plugin install failed", "id", id, "error", err)
			return "", fmt.Errorf("install js plugin %s: %w", id, err)
		}
		m.plugins[id] = p
		m.proxy.SetNeedsJS(id, p.meta.NeedsJS)
		logger.Debug("js plugin installed", "id", id)
		return filepath.Join(destDir, "main.js"), nil
	}

	dest := filepath.Join(m.pluginsDir, id+".wasm")
	logger.Debug("installing plugin", "source", wasmPath, "dest", dest)
	if filepath.Clean(wasmPath) != filepath.Clean(dest) {
		if err := copyFile(wasmPath, dest); err != nil {
			return "", fmt.Errorf("copy plugin %s: %w", id, err)
		}
	}
	p, err := m.load(id, dest)
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
		if p.kind == "wasm" && p.extismPlugin != nil {
			_ = p.extismPlugin.Close(m.ctx)
		}
	}
	return nil
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
	SearchPageSize   int
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
			SearchPageSize:   p.meta.SearchPageSize,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

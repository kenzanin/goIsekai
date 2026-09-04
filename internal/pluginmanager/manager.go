package pluginmanager

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"plugin"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dop251/goja"
	"github.com/extism/go-sdk"

	lunar "github.com/mmcdole/lunar"

	"goisekai/internal/hostnet"
	"goisekai/internal/logger"
	"goisekai/pkg/types"
)

const (
	// memoryLimitPages caps a plugin's linear memory at 64 MB (1024 * 64 KiB).
	memoryLimitPages = 1024
	// invokeTimeout bounds a single plugin invocation (5.3: 15 s).
	invokeTimeout = 15 * time.Second
)

// loadedPlugin is a compiled and instantiated plugin and its resolved ABI
// entry points. kind is "wasm" or "lua"; only the relevant fields are set.
type loadedPlugin struct {
	id       string
	wasmPath string
	kind     string // "wasm", "lua", or "js"
	loaded   bool   // true after the runtime has been instantiated
	// extismPlugin holds the Extism plugin instance for wasm-kind plugins (nil otherwise).
	extismPlugin *extism.Plugin
	// lunar holds the Lunar VM for lua-kind plugins (nil for wasm/js).
	lunar *lunar.State
	// goPlugin holds the opened .so handle for go-kind plugins (nil otherwise).
	goPlugin *plugin.Plugin
	// goFns caches resolved ABI symbols for go-kind plugins.
	goFns map[string]any
	// js holds the goja VM for js-kind plugins (nil otherwise).
	js *goja.Runtime
	// contractVersion is the plugin's resolved contract_version.
	contractVersion int32
	// meta is the metadata the plugin declared in its optional Init export.
	meta types.PluginMeta
	// mu serializes invocations: concurrent calls to the same plugin must not interleave.
	mu sync.Mutex
}

// ensureLoaded lazily instantiates the plugin runtime on first use.
// It is safe to call concurrently; the per-plugin mutex serializes
// multiple callers, and only the first one performs the actual load.
func (m *Manager) ensureLoaded(id string) error {
	m.mu.RLock()
	p, ok := m.plugins[id]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("plugin %q not registered", id)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.loaded {
		return nil
	}
	var loaded *loadedPlugin
	var err error
	switch p.kind {
	case "wasm":
		loaded, err = m.load(p.id, p.wasmPath)
	case "lua":
		loaded, err = m.loadLua(p.id, p.wasmPath)
	case "js":
		loaded, err = m.loadJS(p.id, p.wasmPath)
	case "go":
		loaded, err = m.loadGo(p.id, p.wasmPath)
	default:
		return fmt.Errorf("plugin %q: unknown kind %q", id, p.kind)
	}
	if err != nil {
		return fmt.Errorf("lazy-load plugin %s: %w", id, err)
	}
		p.extismPlugin = loaded.extismPlugin
		p.lunar = loaded.lunar
		p.js = loaded.js
		p.goPlugin = loaded.goPlugin
		p.goFns = loaded.goFns
	p.contractVersion = loaded.contractVersion
	p.meta = loaded.meta
	p.loaded = true
	m.proxy.SetNeedsJS(id, p.meta.NeedsJS)
	logger.Debug("plugin loaded (lazy)", "id", id, "version", p.contractVersion)
	return nil
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

// Discover scans pluginsDir and registers every *.wasm file and every
// folder containing main.lua or main.js, WITHOUT instantiating any runtime.
// Plugins are lazily instantiated on first use via ensureLoaded. A folder
// that collides with an already-registered id is logged and skipped rather
// than aborting discovery.
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
		if _, dup := m.plugins[id]; dup {
			logger.Error("plugin id collision, skipping", "id", id, "kind", "wasm")
			continue
		}
		m.plugins[id] = &loadedPlugin{
			id:       id,
			wasmPath: path,
			kind:     "wasm",
		}
		logger.Debug("plugin registered", "id", id, "path", path)
	}

	// Lua plugins: one folder per plugin, main.lua entry, folder name = id.
	luaMatches, err := filepath.Glob(filepath.Join(m.pluginsDir, "*", "main.lua"))
	if err != nil {
		return err
	}
	for _, path := range luaMatches {
		id := filepath.Base(filepath.Dir(path))
		if _, dup := m.plugins[id]; dup {
			logger.Error("plugin id collision, skipping", "id", id, "kind", "lua")
			continue
		}
		m.plugins[id] = &loadedPlugin{
			id:       id,
			wasmPath: filepath.Dir(path),
			kind:     "lua",
		}
		logger.Debug("lua plugin registered", "id", id, "path", path)
	}

	// JS plugins: one folder per plugin, main.js entry, folder name = id.
	jsMatches, err := filepath.Glob(filepath.Join(m.pluginsDir, "*", "main.js"))
	if err != nil {
		return err
	}
	for _, path := range jsMatches {
		id := filepath.Base(filepath.Dir(path))
		if _, dup := m.plugins[id]; dup {
			logger.Error("plugin id collision, skipping", "id", id, "kind", "js")
			continue
		}
		m.plugins[id] = &loadedPlugin{
			id:       id,
			wasmPath: filepath.Dir(path),
			kind:     "js",
		}
		logger.Debug("js plugin registered", "id", id, "path", path)
	}

	// Go native plugins: one .so file per plugin, filename (minus .so) = id.
	if err := m.discoverGo(); err != nil {
		return err
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

// LoadPlugin hot-loads a single plugin from a file (.wasm) or folder
// (containing main.lua or main.js). The plugin is registered immediately.
func (m *Manager) LoadPlugin(path string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.plugins == nil {
		m.plugins = make(map[string]*loadedPlugin)
	}

	// Determine type by probing for entry files
	mainLua := filepath.Join(path, "main.lua")
	if info, err := os.Stat(mainLua); err == nil && !info.IsDir() {
		id := filepath.Base(path)
		if _, dup := m.plugins[id]; dup {
			return "", fmt.Errorf("plugin %q already loaded", id)
		}
		p, err := m.loadLua(id, path)
		if err != nil {
			return "", err
		}
		m.plugins[id] = p
		m.proxy.SetNeedsJS(id, p.meta.NeedsJS)
		logger.Info("plugin loaded (hot)", "id", id, "kind", "lua")
		return id, nil
	}

	mainJS := filepath.Join(path, "main.js")
	if info, err := os.Stat(mainJS); err == nil && !info.IsDir() {
		id := filepath.Base(path)
		if _, dup := m.plugins[id]; dup {
			return "", fmt.Errorf("plugin %q already loaded", id)
		}
		p, err := m.loadJS(id, path)
		if err != nil {
			return "", err
		}
		m.plugins[id] = p
		m.proxy.SetNeedsJS(id, p.meta.NeedsJS)
		logger.Info("plugin loaded (hot)", "id", id, "kind", "js")
		return id, nil
	}

	// WASM: path must be a .wasm file
	if !strings.HasSuffix(path, ".wasm") {
		return "", fmt.Errorf("no main.lua, main.js, or .wasm found at %s", path)
	}
	id := strings.TrimSuffix(filepath.Base(path), ".wasm")
	if _, dup := m.plugins[id]; dup {
		return "", fmt.Errorf("plugin %q already loaded", id)
	}
	p, err := m.load(id, path)
	if err != nil {
		return "", err
	}
	m.plugins[id] = p
	m.proxy.SetNeedsJS(id, p.meta.NeedsJS)
	logger.Info("plugin loaded (hot)", "id", id, "kind", "wasm")
	return id, nil
}

// UnloadPlugin releases the runtime for a plugin, reverting it to the
// registered-only state. The plugin stays in the manager and database so
// it will be lazily reloaded on next use.
func (m *Manager) UnloadPlugin(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.plugins[id]
	if !ok {
		return fmt.Errorf("plugin %q not loaded", id)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.loaded {
		return nil
	}
	if p.kind == "lua" && p.lunar != nil {
		p.lunar.Close()
	}
	if p.kind == "wasm" && p.extismPlugin != nil {
		_ = p.extismPlugin.Close(m.ctx)
	}
	if p.kind == "js" && p.js != nil {
		p.js.Interrupt("unloading")
	}
	p.extismPlugin = nil
	p.lunar = nil
	p.js = nil
	p.goPlugin = nil
	p.goFns = nil
	p.contractVersion = 0
	p.meta = types.PluginMeta{}
	p.loaded = false
	logger.Info("plugin unloaded", "id", id)
	return nil
}

// ReloadPlugin unloads a plugin and re-loads it from its current path on disk.
func (m *Manager) ReloadPlugin(id string) (string, error) {
	m.mu.Lock()
	old, ok := m.plugins[id]
	if !ok {
		m.mu.Unlock()
		return "", fmt.Errorf("plugin %q not loaded", id)
	}
	// Close the old plugin instance first
	if old.kind == "lua" && old.lunar != nil {
		old.lunar.Close()
	}
	if old.kind == "wasm" && old.extismPlugin != nil {
		_ = old.extismPlugin.Close(m.ctx)
	}
	if old.kind == "js" && old.js != nil {
		old.js.Interrupt("reloading")
	}
	// Go native plugins have no explicit unload in pkg/plugin; dropping the
	// handle leaks the mapped .so until process exit (acceptable on reload).
	delete(m.plugins, id)
	m.mu.Unlock()

	// Determine reload path from stored wasmPath
	var path string
	switch old.kind {
	case "wasm":
		path = old.wasmPath
	case "lua":
		path = filepath.Dir(old.wasmPath)
	case "js":
		path = filepath.Dir(old.wasmPath)
	}
	newID, err := m.LoadPlugin(path)
	if err != nil {
		return "", fmt.Errorf("reload %s: %w", id, err)
	}
	logger.Info("plugin reloaded", "id", newID)
	return newID, nil
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

// Close releases the runtime of every instantiated plugin. Registered-only
// (deferred) plugins have no runtime to release and are skipped.
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, p := range m.plugins {
		if !p.loaded {
			continue
		}
		if p.kind == "lua" && p.lunar != nil {
			p.lunar.Close()
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

// LoadedPlugin is metadata about a currently-registered plugin.
type LoadedPlugin struct {
	ID               string
	Version          string // ABI contract version (e.g. "1")
	Loaded           bool   // true when the runtime is instantiated
	WasmPath         string
	VerifyURL        string // from the plugin's optional Init metadata
	NeedsHumanVerify bool
	ThumbRatio       float64
	NeedsJS          bool
	SearchPageSize   int
}

// LoadedPlugins returns metadata for every plugin currently registered,
// sorted by id.
func (m *Manager) LoadedPlugins() []LoadedPlugin {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]LoadedPlugin, 0, len(m.plugins))
	for _, p := range m.plugins {
		out = append(out, LoadedPlugin{
			ID:               p.id,
			Version:          strconv.Itoa(int(p.contractVersion)),
			Loaded:           p.loaded,
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

package pluginmanager

import (
	"context"
	"plugin"
	"sync"
	"time"

	"github.com/dop251/goja"
	lunar "github.com/mmcdole/lunar"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"

	"goisekai/internal/database"
	"goisekai/internal/enrich"
	"goisekai/internal/hostnet"
	"goisekai/pkg/types"
)

const (
	// invokeTimeout bounds a single plugin invocation (5.3: 15 s).
	invokeTimeout = 15 * time.Second
)

// loadedPlugin is a compiled and instantiated plugin and its resolved ABI
type loadedPlugin struct {
	id       string
	wasmPath string // path to the plugin directory (or main.lua/main.js entry)
	kind     string // "lua", "js", "go", or "yaegi"
	loaded   bool   // true after the runtime has been instantiated
	// lunar holds the Lunar VM for lua-kind plugins.
	lunar *lunar.State
	// goPlugin holds the opened .so handle for go-kind plugins.
	goPlugin *plugin.Plugin
	// goFns caches resolved ABI symbols for go-kind plugins.
	goFns map[string]any
	// js holds the goja VM for js-kind plugins.
	js *goja.Runtime
	// yaegi holds the Yaegi interpreter for yaegi-kind plugins.
	yaegi *yaegiPlugin
	// wasmMod is the instantiated wazero module for wasm-kind plugins.
	wasmMod api.Module
	// wasmFns caches resolved ABI exports for wasm-kind plugins.
	wasmFns map[string]api.Function
	// contractVersion is the plugin's resolved contract_version.
	contractVersion int32
	// meta is the metadata the plugin declared in its optional Init export.
	meta types.PluginMeta
	// infoOnly marks an enrichment script discovered under infoDir. It has no
	// source ABI and is excluded from the manga-source plugin list.
	infoOnly bool
	// mu serializes invocations: concurrent calls to the same plugin must not interleave.
	// It also guards curCtx: the JS runtime's goja natives receive no context, so
	// callJS publishes the invoke ctx here for them to read.
	mu sync.Mutex
	// curCtx is the in-flight invocation's context, set by callJS while mu is
	// held. It carries the invoke deadline so a blocking host call (a CDP solve,
	// an upstream fetch) is torn down when the invoke budget runs out instead of
	// outliving it on context.Background(). Zero value means "no invoke running".
	curCtx context.Context
}

// invokeDeadline returns the per-invocation wall-clock budget: the plugin's
// PLUGIN.timeout (seconds) when declared, else the shared 15 s default.
func (p *loadedPlugin) invokeDeadline() time.Duration {
	if p.meta.Timeout > 0 {
		return time.Duration(p.meta.Timeout) * time.Second
	}
	return invokeTimeout
}

// Manager loads Lua/JS/Yaegi plugins and exposes their search/detail operations
// to the host. It wires host_http_request to the hostnet proxy.
type Manager struct {
	proxy      *hostnet.Proxy
	pluginsDir string
	// infoDir holds enrichment scripts (one folder per source, each with a
	// main.lua). They run in the same sandbox as a plugin but serve metadata
	// instead of scraping a site, so they never appear as a manga source.
	infoDir string
	ctx     context.Context

	mu      sync.RWMutex
	plugins map[string]*loadedPlugin
	// runtime is the shared wazero runtime for all wasm plugins, lazily
	// created by ensureWasmRuntime and closed by Close.
	runtime wazero.Runtime
	onLoad  func(id string)  // called after first successful load
	enrich  *enrich.Registry // optional enrichment registry; plugin providers registered on load

	// db is the database handle for caching plugin responses.
	db *database.DB
	// cacheTTL is the TTL for cached plugin responses.
	// chapterCacheTTL is the TTL for cached chapter lists.
	chapterCacheTTL time.Duration
	cacheTTL        time.Duration
}

// SetOnLoad registers a callback that fires once per plugin after its first
// successful lazy-load. The callback receives the plugin id.
func (m *Manager) SetOnLoad(fn func(id string)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onLoad = fn
}

// invokeCtx returns the context of the in-flight invocation for plugin id, or
// context.Background() when none is running. JS natives run inside the goja VM
// and receive no ctx from the runtime, so they read the ctx published by
// callJS here; without it a blocking host call (HTTP fetch, CDP solve) outlives
// the invoke deadline.
func (m *Manager) invokeCtx(id string) context.Context {
	m.mu.RLock()
	p, ok := m.plugins[id]
	m.mu.RUnlock()
	if !ok || p.curCtx == nil {
		return context.Background()
	}
	return p.curCtx
}

// SetEnrichRegistry wires an enrichment registry so that plugin-declared
// enrichment providers are registered with the registry when the plugin loads.
// If no enrichment registry is provided, this is a no-op.
func (m *Manager) SetEnrichRegistry(reg *enrich.Registry) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.enrich = reg
}

// NewManager returns a Manager that will load plugins from pluginsDir and route
// their network access through proxy. SetDB must be called before use for caching.
func NewManager(proxy *hostnet.Proxy, pluginsDir string) *Manager {
	return &Manager{
		proxy:           proxy,
		pluginsDir:      pluginsDir,
		ctx:             context.Background(),
		plugins:         make(map[string]*loadedPlugin),
		chapterCacheTTL: 168 * time.Hour, // default TTL for chapter lists
		cacheTTL:        24 * time.Hour,  // default TTL for detail
	}
}

// SetInfoDir points the manager at the enrichment script directory. Discovery
// of that directory is part of Discover, so call this before it.
func (m *Manager) SetInfoDir(dir string) {
	m.infoDir = dir
}

// SetDB wires the database handle and cache TTL for response caching.
func (m *Manager) SetDB(db *database.DB, cacheTTL time.Duration) {
	m.db = db
	m.cacheTTL = cacheTTL
}

// SetCacheTTL updates the cache TTL (e.g. from config reload).
func (m *Manager) SetCacheTTL(ttl time.Duration) {
	m.cacheTTL = ttl
}

// SetChapterCacheTTL updates the chapter list cache TTL.
func (m *Manager) SetChapterCacheTTL(ttl time.Duration) {
	m.chapterCacheTTL = ttl
}

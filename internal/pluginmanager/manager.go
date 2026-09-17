package pluginmanager

import (
	"context"
	"plugin"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/dop251/goja"
	lunar "github.com/mmcdole/lunar"

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
	// contractVersion is the plugin's resolved contract_version.
	contractVersion int32
	// meta is the metadata the plugin declared in its optional Init export.
	meta types.PluginMeta
	// infoOnly marks an enrichment script discovered under infoDir. It has no
	// source ABI and is excluded from the manga-source plugin list.
	infoOnly bool
	// mu serializes invocations: concurrent calls to the same plugin must not interleave.
	mu sync.Mutex
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
	onLoad  func(id string)  // called after first successful load
	enrich  *enrich.Registry // optional enrichment registry; plugin providers registered on load

	// db is the database handle for caching plugin responses.
	db *database.DB
	// cacheTTL is the TTL for cached plugin responses.
	cacheTTL time.Duration
}

// SetOnLoad registers a callback that fires once per plugin after its first
// successful lazy-load. The callback receives the plugin id.
func (m *Manager) SetOnLoad(fn func(id string)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onLoad = fn
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
		proxy:      proxy,
		pluginsDir: pluginsDir,
		ctx:        context.Background(),
		plugins:    make(map[string]*loadedPlugin),
		cacheTTL:   24 * time.Hour, // default TTL
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

// LoadedPlugin is metadata about a currently-registered plugin.
type LoadedPlugin struct {
	ID               string
	Kind             string // runtime: "lua", "js", "go" or "yaegi"
	Version          string // ABI contract version (e.g. "1")
	Loaded           bool   // true when the runtime is instantiated
	WasmPath         string // path to the plugin directory or entry file
	VerifyURL        string // from the plugin's optional Init metadata
	NeedsHumanVerify bool
	ThumbRatio       float64
	NeedsJS          bool
	Name             string
	SiteURL          string
	Logo             string
}

// LoadedPlugins returns metadata for every plugin currently registered,
// sorted by id.
func (m *Manager) LoadedPlugins() []LoadedPlugin {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]LoadedPlugin, 0, len(m.plugins))
	for _, p := range m.plugins {
		if p.infoOnly {
			// An enrichment script is not a manga source: keep it out of the
			// plugin list the UI and search read.
			continue
		}
		out = append(out, LoadedPlugin{
			ID:               p.id,
			Kind:             p.kind,
			Version:          strconv.Itoa(int(p.contractVersion)),
			Loaded:           p.loaded,
			WasmPath:         p.wasmPath,
			VerifyURL:        p.meta.VerifyURL,
			NeedsHumanVerify: p.meta.NeedsHumanVerify,
			ThumbRatio:       p.meta.ThumbRatio,
			NeedsJS:          p.meta.NeedsJS,
			Name:             p.meta.Name,
			SiteURL:          p.meta.SiteURL,
			Logo:             p.meta.Logo,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Proxy returns the internal proxy (for testing/debugging only)
func (m *Manager) Proxy() *hostnet.Proxy {
	return m.proxy
}

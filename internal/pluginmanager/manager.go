package pluginmanager

import (
	"context"
	"plugin"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/dop251/goja"
	extism "github.com/extism/go-sdk"

	lunar "github.com/mmcdole/lunar"

	"goisekai/internal/hostnet"
	"goisekai/pkg/types"
)

const (
	// memoryLimitPages caps a plugin's linear memory at 64 MB (1024 * 64 KiB).
	memoryLimitPages = 1024
	// invokeTimeout bounds a single plugin invocation (5.3: 15 s).
	invokeTimeout = 15 * time.Second
)

// loadedPlugin is a compiled and instantiated plugin and its resolved ABI
// entry points. kind is "wasm", "lua", "js", "go", or "scriggo"; only the
// relevant fields are set.
type loadedPlugin struct {
	id       string
	wasmPath string
	kind     string // "wasm", "lua", "js", "go", or "scriggo"
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
	// scriggo holds the compiled Scriggo program for scriggo-kind plugins (nil otherwise).
	scriggo *scriggoPlugin
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
	onLoad  func(id string) // called after first successful load
}

// SetOnLoad registers a callback that fires once per plugin after its first
// successful lazy-load. The callback receives the plugin id.
func (m *Manager) SetOnLoad(fn func(id string)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onLoad = fn
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
			Name:             p.meta.Name,
			SiteURL:          p.meta.SiteURL,
			Logo:             p.meta.Logo,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

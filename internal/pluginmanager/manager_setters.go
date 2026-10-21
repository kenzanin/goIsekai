package pluginmanager

import (
	"context"
	"time"

	"goisekai/internal/database"
	"goisekai/internal/enrich"
	"goisekai/internal/hostnet"
)

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

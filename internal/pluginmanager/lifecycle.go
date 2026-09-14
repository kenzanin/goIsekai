package pluginmanager

import (
	"net/url"
	"fmt"
	"goisekai/internal/enrich"
	"goisekai/internal/logger"
)

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
	case "lua":
		loaded, err = m.loadLua(p.id, p.wasmPath)
	case "js":
		loaded, err = m.loadJS(p.id, p.wasmPath)
	case "go":
		loaded, err = m.loadGo(p.id, p.wasmPath)
	case "yaegi":
		loaded, err = m.loadYaegi(p.id, p.wasmPath)
	default:
		return fmt.Errorf("plugin %q: unknown kind %q", id, p.kind)
	}
	if err != nil {
		return fmt.Errorf("lazy-load plugin %s: %w", id, err)
	}
	p.lunar = loaded.lunar
	p.js = loaded.js
	p.goPlugin = loaded.goPlugin
	p.goFns = loaded.goFns
	p.yaegi = loaded.yaegi
	p.contractVersion = loaded.contractVersion
	p.meta = loaded.meta
	p.loaded = true
	m.proxy.SetNeedsJS(id, p.meta.NeedsJS)
	m.proxy.SetHTTPProfiles(id, p.meta.HTTPProfiles)
	logger.Info("plugin loaded (lazy)", "id", id, "kind", p.kind, "version", p.contractVersion)

	// Register plugin-declared enrichment providers so they're visible
	// in the catalog as soon as the plugin is first invoked.
	if m.enrich != nil {
		for _, ep := range p.meta.EnrichmentProviders {
			ks := make([]enrich.Kind, len(ep.Kinds))
			for i, k := range ep.Kinds {
				ks[i] = enrich.Kind(k)
			}
			m.enrich.Register(&pluginProvider{
				pluginID: id,
				id:       ep.ID,
				name:     ep.Name,
				kinds:    ks,
				fetch:    m,
			})
		}
	}

	if m.onLoad != nil {
		go m.onLoad(id)
		go m.preconnectOnLoad(id)
	}
	return nil
}

// preconnectOnLoad triggers preconnect in the background after plugin lazy-load.
// It extracts the host from the plugin site_url and initiates a preconnect.
func (m *Manager) preconnectOnLoad(id string) {
	p, err := m.get(id)
	if err != nil {
		logger.Debug("preconnectOnLoad failed", "plugin", id, "error", err)
		return
	}
	if p.meta.SiteURL == "" {
		return
	}
	// Extract host from URL
	u, err := url.Parse(p.meta.SiteURL)
	if err != nil {
		logger.Debug("preconnectOnLoad parse URL", "plugin", id, "url", p.meta.SiteURL, "error", err)
		return
	}
	m.proxy.Preconnect(u.Host)
}

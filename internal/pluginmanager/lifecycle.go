package pluginmanager

import (
	"fmt"
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
	case "wasm":
		loaded, err = m.load(p.id, p.wasmPath)
	case "lua":
		loaded, err = m.loadLua(p.id, p.wasmPath)
	case "js":
		loaded, err = m.loadJS(p.id, p.wasmPath)
	case "go":
		loaded, err = m.loadGo(p.id, p.wasmPath)
	case "scriggo":
		loaded, err = m.loadScriggo(p.id, p.wasmPath)
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
	p.scriggo = loaded.scriggo
	p.contractVersion = loaded.contractVersion
	p.meta = loaded.meta
	p.loaded = true
	m.proxy.SetNeedsJS(id, p.meta.NeedsJS)
	m.proxy.SetHTTPProfiles(id, p.meta.HTTPProfiles)
	logger.Debug("plugin loaded (lazy)", "id", id, "version", p.contractVersion)
	if m.onLoad != nil {
		go m.onLoad(id)
	}
	return nil
}

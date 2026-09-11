package pluginmanager

import (
	"goisekai/internal/logger"
	"path/filepath"
)

// Discover scans pluginsDir and registers every folder containing main.lua,
// main.js, or main.go, WITHOUT instantiating any runtime. Plugins are lazily
// instantiated on first use via ensureLoaded. A folder that collides with an
// already-registered id is logged and skipped rather than aborting discovery.
func (m *Manager) Discover() error {
	m.mu.Lock()
	defer m.mu.Unlock()

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

	// Yaegi plugins: one folder per plugin, main.go entry, folder name = id.
	yaegiMatches, err := filepath.Glob(filepath.Join(m.pluginsDir, "*", "main.go"))
	if err != nil {
		return err
	}
	for _, path := range yaegiMatches {
		id := filepath.Base(filepath.Dir(path))
		if _, dup := m.plugins[id]; dup {
			logger.Error("plugin id collision, skipping", "id", id, "kind", "yaegi")
			continue
		}
		m.plugins[id] = &loadedPlugin{
			id:       id,
			wasmPath: filepath.Dir(path),
			kind:     "yaegi",
		}
		logger.Debug("yaegi plugin registered", "id", id, "path", path)
	}

	// Go native plugins: one .so file per plugin, filename (minus .so) = id.
	if err := m.discoverGo(); err != nil {
		return err
	}
	return nil
}

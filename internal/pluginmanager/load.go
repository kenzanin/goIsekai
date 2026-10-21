package pluginmanager

import (
	"fmt"
	"goisekai/internal/logger"
	"os"
	"path/filepath"
)

// LoadPlugin hot-loads a single plugin folder (containing main.lua, main.js, or main.go).
// The plugin is registered immediately.
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
		m.registerLoaded(id, p)
		logger.Debug("plugin loaded (hot)", "id", id, "kind", "lua")
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
		m.registerLoaded(id, p)
		logger.Debug("plugin loaded (hot)", "id", id, "kind", "js")
		return id, nil
	}

	mainGo := filepath.Join(path, "main.go")
	if info, err := os.Stat(mainGo); err == nil && !info.IsDir() {
		id := filepath.Base(path)
		if _, dup := m.plugins[id]; dup {
			return "", fmt.Errorf("plugin %q already loaded", id)
		}
		p, err := m.loadYaegi(id, path)
		if err != nil {
			return "", err
		}
		m.registerLoaded(id, p)
		logger.Debug("plugin loaded (hot)", "id", id, "kind", "yaegi")
		return id, nil
	}

	mainWasm := filepath.Join(path, "main.wasm")
	if info, err := os.Stat(mainWasm); err == nil && !info.IsDir() {
		id := filepath.Base(path)
		if _, dup := m.plugins[id]; dup {
			return "", fmt.Errorf("plugin %q already loaded", id)
		}
		p, err := m.loadWasm(id, mainWasm)
		if err != nil {
			return "", err
		}
		m.registerLoaded(id, p)
		logger.Debug("plugin loaded (hot)", "id", id, "kind", "wasm")
		return id, nil
	}

	mainSo := filepath.Join(path, "main.so")
	if info, err := os.Stat(mainSo); err == nil && !info.IsDir() {
		id := filepath.Base(path)
		if _, dup := m.plugins[id]; dup {
			return "", fmt.Errorf("plugin %q already loaded", id)
		}
		p, err := m.loadGo(id, mainSo)
		if err != nil {
			return "", err
		}
		m.registerLoaded(id, p)
		logger.Debug("plugin loaded (hot)", "id", id, "kind", "go")
		return id, nil
	}

	return "", fmt.Errorf("no main.lua, main.js, main.go, main.wasm, or main.so found at %s", path)
}

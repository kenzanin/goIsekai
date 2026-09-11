package pluginmanager

import (
	"fmt"
	"goisekai/internal/logger"
	"os"
	"path/filepath"
)

// Install copies a plugin folder (lua, js, or yaegi containing main.lua/main.js/main.go)
// into pluginsDir, hot-loads it, and registers it under its base name. It
// must be called after Discover. It returns the path of the copy inside
// pluginsDir (stored as WasmPath in the DB).
func (m *Manager) Install(dirPath string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.plugins == nil {
		return "", fmt.Errorf("Discover must be called before Install")
	}
	var id string

	// Lua plugin: source is a folder containing main.lua; copy it recursively.
	mainLua := filepath.Join(dirPath, "main.lua")
	if info, err := os.Stat(mainLua); err == nil && !info.IsDir() {
		id = filepath.Base(dirPath)
		destDir := filepath.Join(m.pluginsDir, id)
		logger.Debug("installing lua plugin", "source", dirPath, "dest", destDir)
		if filepath.Clean(dirPath) != filepath.Clean(destDir) {
			if err := copyDir(dirPath, destDir); err != nil {
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
		m.proxy.SetHTTPProfiles(id, p.meta.HTTPProfiles)
		logger.Debug("lua plugin installed", "id", id)
		return filepath.Join(destDir, "main.lua"), nil
	}

	// JS plugin: source is a folder containing main.js; copy it recursively.
	mainJS := filepath.Join(dirPath, "main.js")
	if info, err := os.Stat(mainJS); err == nil && !info.IsDir() {
		id = filepath.Base(dirPath)
		destDir := filepath.Join(m.pluginsDir, id)
		logger.Debug("installing js plugin", "source", dirPath, "dest", destDir)
		if filepath.Clean(dirPath) != filepath.Clean(destDir) {
			if err := copyDir(dirPath, destDir); err != nil {
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
		m.proxy.SetHTTPProfiles(id, p.meta.HTTPProfiles)
		logger.Debug("js plugin installed", "id", id)
		return filepath.Join(destDir, "main.js"), nil
	}

	// Yaegi plugin: source is a folder containing main.go; copy it recursively.
	mainGo := filepath.Join(dirPath, "main.go")
	if info, err := os.Stat(mainGo); err == nil && !info.IsDir() {
		id = filepath.Base(dirPath)
		destDir := filepath.Join(m.pluginsDir, id)
		logger.Debug("installing yaegi plugin", "source", dirPath, "dest", destDir)
		if filepath.Clean(dirPath) != filepath.Clean(destDir) {
			if err := copyDir(dirPath, destDir); err != nil {
				return "", fmt.Errorf("copy yaegi plugin %s: %w", id, err)
			}
		}
		p, err := m.loadYaegi(id, destDir)
		if err != nil {
			logger.Error("yaegi plugin install failed", "id", id, "error", err)
			return "", fmt.Errorf("install yaegi plugin %s: %w", id, err)
		}
		m.plugins[id] = p
		m.proxy.SetNeedsJS(id, p.meta.NeedsJS)
		m.proxy.SetHTTPProfiles(id, p.meta.HTTPProfiles)
		logger.Debug("yaegi plugin installed", "id", id)
		return filepath.Join(destDir, "main.go"), nil
	}

	return "", fmt.Errorf("no main.lua, main.js, or main.go found at %s", dirPath)
}

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
		m.plugins[id] = p
		m.proxy.SetNeedsJS(id, p.meta.NeedsJS)
		m.proxy.SetHTTPProfiles(id, p.meta.HTTPProfiles)
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
		m.proxy.SetHTTPProfiles(id, p.meta.HTTPProfiles)
		logger.Info("plugin loaded (hot)", "id", id, "kind", "js")
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
		m.plugins[id] = p
		m.proxy.SetNeedsJS(id, p.meta.NeedsJS)
		m.proxy.SetHTTPProfiles(id, p.meta.HTTPProfiles)
		logger.Info("plugin loaded (hot)", "id", id, "kind", "yaegi")
		return id, nil
	}

	return "", fmt.Errorf("no main.lua, main.js, or main.go found at %s", path)
}

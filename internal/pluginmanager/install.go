package pluginmanager

import (
	"fmt"
	"goisekai/internal/logger"
	"os"
	"path/filepath"
	"strings"
)

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
		m.proxy.SetHTTPProfiles(id, p.meta.HTTPProfiles)
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
		m.proxy.SetHTTPProfiles(id, p.meta.HTTPProfiles)
		logger.Debug("js plugin installed", "id", id)
		return filepath.Join(destDir, "main.js"), nil
	}

	// Scriggo plugin: source is a folder containing main.go; copy it recursively.
	mainGo := filepath.Join(wasmPath, "main.go")
	if info, err := os.Stat(mainGo); err == nil && !info.IsDir() {
		id = filepath.Base(wasmPath)
		destDir := filepath.Join(m.pluginsDir, id)
		logger.Debug("installing scriggo plugin", "source", wasmPath, "dest", destDir)
		if filepath.Clean(wasmPath) != filepath.Clean(destDir) {
			if err := copyDir(wasmPath, destDir); err != nil {
				return "", fmt.Errorf("copy scriggo plugin %s: %w", id, err)
			}
		}
		p, err := m.loadScriggo(id, destDir)
		if err != nil {
			logger.Error("scriggo plugin install failed", "id", id, "error", err)
			return "", fmt.Errorf("install scriggo plugin %s: %w", id, err)
		}
		m.plugins[id] = p
		m.proxy.SetNeedsJS(id, p.meta.NeedsJS)
		m.proxy.SetHTTPProfiles(id, p.meta.HTTPProfiles)
		logger.Debug("scriggo plugin installed", "id", id)
		return filepath.Join(destDir, "main.go"), nil
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
	m.proxy.SetHTTPProfiles(id, p.meta.HTTPProfiles)
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
		p, err := m.loadScriggo(id, path)
		if err != nil {
			return "", err
		}
		m.plugins[id] = p
		m.proxy.SetNeedsJS(id, p.meta.NeedsJS)
		m.proxy.SetHTTPProfiles(id, p.meta.HTTPProfiles)
		logger.Info("plugin loaded (hot)", "id", id, "kind", "scriggo")
		return id, nil
	}

	// WASM: path must be a .wasm file
	if !strings.HasSuffix(path, ".wasm") {
		return "", fmt.Errorf("no main.lua, main.js, main.go, or .wasm found at %s", path)
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
	m.proxy.SetHTTPProfiles(id, p.meta.HTTPProfiles)
	logger.Info("plugin loaded (hot)", "id", id, "kind", "wasm")
	return id, nil
}

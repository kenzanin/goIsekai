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
		m.registerLoaded(id, p)
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
		m.registerLoaded(id, p)
		logger.Debug("js plugin installed", "id", id)
		return filepath.Join(destDir, "main.js"), nil
	}

	// WASM plugin: source is a folder containing main.wasm; copy it recursively.
	mainWasm := filepath.Join(dirPath, "main.wasm")
	if info, err := os.Stat(mainWasm); err == nil && !info.IsDir() {
		id := filepath.Base(dirPath)
		destDir := filepath.Join(m.pluginsDir, id)
		logger.Debug("installing wasm plugin", "source", dirPath, "dest", destDir)
		if filepath.Clean(dirPath) != filepath.Clean(destDir) {
			if err := copyDir(dirPath, destDir); err != nil {
				return "", fmt.Errorf("copy wasm plugin %s: %w", id, err)
			}
		}
		p, err := m.loadWasm(id, filepath.Join(destDir, "main.wasm"))
		if err != nil {
			logger.Error("wasm plugin install failed", "id", id, "error", err)
			return "", fmt.Errorf("install wasm plugin %s: %w", id, err)
		}
		m.registerLoaded(id, p)
		logger.Debug("wasm plugin installed", "id", id)
		return filepath.Join(destDir, "main.wasm"), nil
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
		m.registerLoaded(id, p)
		logger.Debug("yaegi plugin installed", "id", id)
		return filepath.Join(destDir, "main.go"), nil
	}

	// Native plugin: source is a folder containing main.so; copy it recursively.
	// Probed last, mirroring discoverGo: a native plugin folder holds only
	// main.so, so nothing above can claim it.
	mainSo := filepath.Join(dirPath, "main.so")
	if info, err := os.Stat(mainSo); err == nil && !info.IsDir() {
		id := filepath.Base(dirPath)
		destDir := filepath.Join(m.pluginsDir, id)
		logger.Debug("installing native plugin", "source", dirPath, "dest", destDir)
		if filepath.Clean(dirPath) != filepath.Clean(destDir) {
			if err := copyDir(dirPath, destDir); err != nil {
				return "", fmt.Errorf("copy native plugin %s: %w", id, err)
			}
		}
		p, err := m.loadGo(id, filepath.Join(destDir, "main.so"))
		if err != nil {
			logger.Error("native plugin install failed", "id", id, "error", err)
			return "", fmt.Errorf("install native plugin %s: %w", id, err)
		}
		m.registerLoaded(id, p)
		logger.Debug("native plugin installed", "id", id)
		return filepath.Join(destDir, "main.so"), nil
	}

	return "", fmt.Errorf("no main.lua, main.js, main.go, main.wasm, or main.so found at %s", dirPath)
}

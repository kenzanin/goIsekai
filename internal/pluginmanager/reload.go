package pluginmanager

import (
	"fmt"
	"goisekai/internal/logger"
	"goisekai/pkg/types"
	"io"
	"os"
	"path/filepath"
)

// UnloadPlugin releases the runtime for a plugin, reverting it to the
// registered-only state. The plugin stays in the manager and database so
// it will be lazily reloaded on next use.
func (m *Manager) UnloadPlugin(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.plugins[id]
	if !ok {
		return fmt.Errorf("plugin %q not loaded", id)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.loaded {
		return nil
	}
	if p.kind == "lua" && p.lunar != nil {
		_ = p.lunar.Close()
	}
	if p.kind == "wasm" && p.extismPlugin != nil {
		_ = p.extismPlugin.Close(m.ctx)
	}
	if p.kind == "js" && p.js != nil {
		p.js.Interrupt("unloading")
	}
	p.extismPlugin = nil
	p.lunar = nil
	p.js = nil
	p.goPlugin = nil
	p.goFns = nil
	p.scriggo = nil
	p.contractVersion = 0
	p.meta = types.PluginMeta{}
	p.loaded = false
	logger.Info("plugin unloaded", "id", id)
	return nil
}

// ReloadPlugin unloads a plugin and re-loads it from its current path on disk.
func (m *Manager) ReloadPlugin(id string) (string, error) {
	m.mu.Lock()
	old, ok := m.plugins[id]
	if !ok {
		m.mu.Unlock()
		return "", fmt.Errorf("plugin %q not loaded", id)
	}
	// Close the old plugin instance first
	if old.kind == "lua" && old.lunar != nil {
		_ = old.lunar.Close()
	}
	if old.kind == "wasm" && old.extismPlugin != nil {
		_ = old.extismPlugin.Close(m.ctx)
	}
	if old.kind == "js" && old.js != nil {
		old.js.Interrupt("reloading")
	}
	// Go native plugins have no explicit unload in pkg/plugin; dropping the
	// handle leaks the mapped .so until process exit (acceptable on reload).
	delete(m.plugins, id)
	m.mu.Unlock()

	// Determine reload path from stored wasmPath.
	// wasm stores the .wasm file path; lua/js/scriggo store their plugin folder
	// directly, so the stored path is already LoadPlugin-ready for every kind.
	path := old.wasmPath
	newID, err := m.LoadPlugin(path)
	if err != nil {
		return "", fmt.Errorf("reload %s: %w", id, err)
	}
	logger.Info("plugin reloaded", "id", newID)
	return newID, nil
}

// copyDir recursively copies src dir to dst (lua plugin folders).
func copyDir(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	for _, e := range entries {
		s := filepath.Join(src, e.Name())
		d := filepath.Join(dst, e.Name())
		if e.IsDir() {
			if err := copyDir(s, d); err != nil {
				return err
			}
			continue
		}
		if err := copyFile(s, d); err != nil {
			return err
		}
	}
	return nil
}

// Close releases the runtime of every instantiated plugin. Registered-only
// (deferred) plugins have no runtime to release and are skipped.
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, p := range m.plugins {
		if !p.loaded {
			continue
		}
		if p.kind == "lua" && p.lunar != nil {
			_ = p.lunar.Close()
		}
		if p.kind == "wasm" && p.extismPlugin != nil {
			_ = p.extismPlugin.Close(m.ctx)
		}
	}
	return nil
}

// copyFile copies src to dst, truncating dst if it exists.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

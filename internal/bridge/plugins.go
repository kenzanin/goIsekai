package bridge

import (
	"fmt"
	"path/filepath"
	"strings"

	"goisekai/internal/database"
)

// InstallPlugin copies a plugin wasm into the managed plugins directory,
// hot-loads it, and registers it in the database as active. The plugin id is
// derived from the file basename (minus the .wasm extension); WasmPath points
// at the copy inside the plugins directory so it survives a restart.
func (s *AppService) InstallPlugin(wasmPath string) error {
	dest, err := s.mgr.Install(wasmPath)
	if err != nil {
		return fmt.Errorf("bridge: install plugin: %w", err)
	}
	id := strings.TrimSuffix(filepath.Base(dest), ".wasm")
	if err := s.db.RegisterPlugin(database.Plugin{
		ID:       id,
		Name:     id,
		Version:  "",
		WasmPath: dest,
		IsActive: true,
	}); err != nil {
		return fmt.Errorf("bridge: register plugin: %w", err)
	}
	return nil
}

// TogglePlugin flips the is_active flag for a plugin.
func (s *AppService) TogglePlugin(id string) error {
	return s.db.TogglePluginActive(id)
}

// LoadPluginHot hot-loads a plugin from an external path without restart.
func (s *AppService) LoadPluginHot(path string) (string, error) {
	return s.mgr.LoadPlugin(path)
}

// UnloadPlugin removes a plugin from memory (files stay on disk).
func (s *AppService) UnloadPlugin(id string) error {
	return s.mgr.UnloadPlugin(id)
}

// ReloadPlugin unloads and re-loads a plugin from its current disk path.
func (s *AppService) ReloadPlugin(id string) (string, error) {
	return s.mgr.ReloadPlugin(id)
}

// ListPlugins returns all registered plugins.
func (s *AppService) ListPlugins() ([]database.Plugin, error) {
	list, err := s.db.ListPlugins()
	if err != nil {
		return nil, fmt.Errorf("bridge: list plugins: %w", err)
	}
	return list, nil
}

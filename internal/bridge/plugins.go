package bridge

import (
	"encoding/json"
	"fmt"
	"goisekai/internal/database"
	"goisekai/internal/logger"
	"goisekai/internal/pluginmanager"
	"path/filepath"
)

// InstallPlugin copies a plugin folder into the managed plugins directory,
// hot-loads it, and registers it in the database as active. The plugin id is
// derived from the folder basename; WasmPath points
// at the copy inside the plugins directory so it survives a restart.
func (s *AppService) InstallPlugin(dirPath string) error {
	dest, err := s.mgr.Install(dirPath)
	if err != nil {
		return fmt.Errorf("bridge: install plugin: %w", err)
	}
	id := filepath.Base(dest)
	if err := s.db.RegisterPlugin(database.Plugin{
		ID:       id,
		Name:     id,
		Version:  "",
		WasmPath: dest,
		IsActive: true,
	}); err != nil {
		return fmt.Errorf("bridge: register plugin: %w", err)
	}
	// Warm the genre cache so the first search page render is instant. A
	// failure here is non-fatal: ListGenres self-heals on first use.
	if _, err := s.ListGenres(id); err != nil {
		logger.Warn("genre cache warmup failed", "plugin", id, "error", err)
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

// Genre re-exports the plugin manager's genre descriptor for views.
type Genre = pluginmanager.Genre

// ListGenres returns the cached genre list for a plugin, normalized to
// canonical name spellings (slugs stay verbatim — they are the search
// filter tokens). Cache states in plugins.genres: NULL = not fetched yet
// (fetch + persist now, one live plugin call ever), "[]" = plugin has no
// genre export (never re-invoke), JSON array = the cached list.
func (s *AppService) ListGenres(pluginID string) ([]Genre, error) {
	if cached, found, err := s.db.GetPluginGenres(pluginID); err != nil {
		return nil, fmt.Errorf("bridge: read genre cache: %w", err)
	} else if found {
		return decodeGenreCache(pluginID, cached)
	}
	genres, err := s.mgr.GetGenres(pluginID)
	if err != nil {
		return nil, err
	}
	// Normalize names one at a time so dedup can't desync name↔slug pairs.
	norm := make([]Genre, 0, len(genres))
	for _, g := range genres {
		name := g.Name
		if fixed := s.genres.normalize([]string{name}); len(fixed) > 0 {
			name = fixed[0]
		} else {
			name = ""
		}
		if name == "" {
			continue
		}
		norm = append(norm, Genre{Name: name, Slug: g.Slug})
	}
	b, mErr := json.Marshal(norm)
	if mErr == nil {
		if err := s.db.SetPluginGenres(pluginID, string(b)); err != nil {
			logger.Warn("genre cache persist failed", "plugin", pluginID, "error", err)
		}
	}
	return norm, nil
}

// decodeGenreCache parses a cached genre JSON payload. A corrupt payload is
// logged and treated as a miss-safe empty list rather than failing the view.
func decodeGenreCache(pluginID, raw string) ([]Genre, error) {
	var out []Genre
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		logger.Warn("genre cache corrupt, ignoring", "plugin", pluginID, "error", err)
		return []Genre{}, nil
	}
	return out, nil
}

// ListPlugins returns all registered plugins.
func (s *AppService) ListPlugins() ([]database.Plugin, error) {
	list, err := s.db.ListPlugins()
	if err != nil {
		return nil, fmt.Errorf("bridge: list plugins: %w", err)
	}
	return list, nil
}

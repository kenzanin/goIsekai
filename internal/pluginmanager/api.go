package pluginmanager

import (
	"fmt"
	"github.com/goccy/go-json"

	"goisekai/pkg/types"
)

// get returns the loaded plugin for pluginID under a read lock.
func (m *Manager) get(pluginID string) (*loadedPlugin, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	p, ok := m.plugins[pluginID]
	if !ok {
		return nil, fmt.Errorf("plugin %q not loaded", pluginID)
	}
	return p, nil
}

// call invokes one JSON-in/JSON-out ABI function on a plugin, enforcing the
// per-invocation timeout. A panic or trap inside the plugin surfaces as an
// error here rather than crashing the host.
func (m *Manager) call(p *loadedPlugin, fnName, inputJSON string) (string, error) {
	if err := m.ensureLoaded(p.id); err != nil {
		return "", err
	}
	if p.kind == "lua" {
		return callLua(p, fnName, inputJSON)
	}
	if p.kind == "js" {
		return callJS(p, fnName, inputJSON)
	}
	if p.kind == "go" {
		return callGo(p, fnName, inputJSON)
	}
	if p.kind == "yaegi" {
		return callYaegi(m, p, fnName, inputJSON)
	}
	if p.kind == "wasm" {
		return callWasm(p, fnName, inputJSON)
	}
	return "", fmt.Errorf("plugin %s %s: unsupported kind %q", p.id, fnName, p.kind)
}

// Search runs a plugin's Search function and decodes its result.
func (m *Manager) Search(pluginID string, filter types.SearchFilter) ([]types.Manga, error) {
	p, err := m.get(pluginID)
	if err != nil {
		return nil, err
	}
	in, err := json.Marshal(filter)
	if err != nil {
		return nil, err
	}
	out, err := m.call(p, types.SearchFunc, string(in))
	if err != nil {
		return nil, err
	}
	var result []types.Manga
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		return nil, fmt.Errorf("plugin %s: invalid Search result: %w", pluginID, err)
	}
	return result, nil
}

// Genre is one browsable genre advertised by a plugin's optional GetGenres
// export. Slug is what a plugin expects in SearchFilter.Genres.
type Genre struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}

// GetGenres runs a plugin's optional GetGenres export. Plugins without the
// export return nil, nil so callers can treat genre browsing as unsupported.
func (m *Manager) GetGenres(pluginID string) ([]Genre, error) {
	p, err := m.get(pluginID)
	if err != nil {
		return nil, err
	}
	out, err := m.call(p, types.GetGenresFunc, "{}")
	if err != nil {
		// Optional export: absent function or unsupported kind means "none".
		return nil, nil
	}
	var result []Genre
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		return nil, fmt.Errorf("plugin %s: invalid GetGenres result: %w", pluginID, err)
	}
	return result, nil
}

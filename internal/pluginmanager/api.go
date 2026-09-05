package pluginmanager

import (
	"encoding/json"
	"fmt"
	"goisekai/internal/logger"
	"sort"

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
	if p.kind == "scriggo" {
		return callScriggo(p, fnName, inputJSON)
	}
	p.mu.Lock()
	defer p.mu.Unlock()

	_, out, err := p.extismPlugin.Call(fnName, []byte(inputJSON))
	if err != nil {
		return "", fmt.Errorf("plugin %s %s: %w", p.id, fnName, err)
	}
	return string(out), nil
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

// GetMangaDetail runs a plugin's GetMangaDetail function and decodes its result.
func (m *Manager) GetMangaDetail(pluginID, mangaID string) (types.Manga, error) {
	p, err := m.get(pluginID)
	if err != nil {
		return types.Manga{}, err
	}
	in, err := json.Marshal(mangaID)
	if err != nil {
		return types.Manga{}, err
	}
	out, err := m.call(p, types.GetMangaDetailFunc, string(in))
	if err != nil {
		return types.Manga{}, err
	}
	var result types.Manga
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		return types.Manga{}, fmt.Errorf("plugin %s: invalid GetMangaDetail result: %w", pluginID, err)
	}
	return result, nil
}

// GetChapterList runs a plugin's GetChapterList function and decodes its result.
func (m *Manager) GetChapterList(pluginID, mangaID string) ([]types.Chapter, error) {
	p, err := m.get(pluginID)
	if err != nil {
		return nil, err
	}
	in, err := json.Marshal(mangaID)
	if err != nil {
		return nil, err
	}
	out, err := m.call(p, types.GetChapterListFunc, string(in))
	if err != nil {
		return nil, err
	}
	var result []types.Chapter
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		return nil, fmt.Errorf("plugin %s: invalid GetChapterList result: %w", pluginID, err)
	}
	return result, nil
}

// GetPageList runs a plugin's GetPageList function and decodes its result.
func (m *Manager) GetPageList(pluginID, chapterID string) ([]types.Page, error) {
	p, err := m.get(pluginID)
	if err != nil {
		return nil, err
	}
	in, err := json.Marshal(chapterID)
	if err != nil {
		return nil, err
	}
	out, err := m.call(p, types.GetPageListFunc, string(in))
	if err != nil {
		return nil, err
	}
	var result []types.Page
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		return nil, fmt.Errorf("plugin %s: invalid GetPageList result: %w", pluginID, err)
	}
	return result, nil
}

// AltTitleServerEntry is one row in the aggregated server list: the plugin
// that provides the server, the server id, and the human-readable name.
type AltTitleServerEntry struct {
	ProviderPluginID string
	ServerID         string
	Name             string
}

// AltTitleServers iterates all discovered plugins and returns every declared
// alt-title server. JS plugins are cheap to load (goja VM, no multi-MB
// runtime), so a deferred JS plugin is loaded on demand to read its metadata;
// WASM/Lua plugins contribute only when already loaded.
func (m *Manager) AltTitleServers() []AltTitleServerEntry {
	// Ensure JS plugins expose their metadata.
	m.mu.RLock()
	var jsIDs []string
	for id, p := range m.plugins {
		if p.kind == "js" && !p.loaded {
			jsIDs = append(jsIDs, id)
		}
	}
	m.mu.RUnlock()
	for _, id := range jsIDs {
		if err := m.ensureLoaded(id); err != nil {
			logger.Warn("alt-title meta load", "id", id, "error", err)
		}
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []AltTitleServerEntry
	for id, p := range m.plugins {
		for _, s := range p.meta.AltTitleServers {
			out = append(out, AltTitleServerEntry{
				ProviderPluginID: id,
				ServerID:         s.ID,
				Name:             s.Name,
			})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].ProviderPluginID != out[j].ProviderPluginID {
			return out[i].ProviderPluginID < out[j].ProviderPluginID
		}
		return out[i].ServerID < out[j].ServerID
	})
	return out
}

// GetAltTitles calls the provider plugin that advertises the given server to
// resolve alternative titles. The input JSON sent to the plugin is
// {"title":..., "server":...} per the ABI contract.
func (m *Manager) GetAltTitles(title, server string) (types.AltTitlesResult, error) {
	// Find the provider plugin for this server.
	m.mu.RLock()
	var providerID string
	for id, p := range m.plugins {
		for _, s := range p.meta.AltTitleServers {
			if s.ID == server {
				providerID = id
				break
			}
		}
		if providerID != "" {
			break
		}
	}
	m.mu.RUnlock()

	if providerID == "" {
		return types.AltTitlesResult{}, fmt.Errorf("server %q not found in any provider", server)
	}

	p, err := m.get(providerID)
	if err != nil {
		return types.AltTitlesResult{}, err
	}

	input, err := json.Marshal(map[string]string{"title": title, "server": server})
	if err != nil {
		return types.AltTitlesResult{}, err
	}
	out, err := m.call(p, types.GetAltTitlesFunc, string(input))
	if err != nil {
		return types.AltTitlesResult{}, err
	}

	var res types.AltTitlesResult
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		return types.AltTitlesResult{}, fmt.Errorf("alt-titles decode: %w", err)
	}
	if res.Source == "" {
		res.Source = providerID
	}
	return res, nil
}

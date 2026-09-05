package pluginmanager

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/dop251/goja"
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

// AltTitlesResult is what a GetAltTitles provider returns: the provider's own
// display name (reported by the plugin, never hardcoded by the host) plus the
// alternative title list.
type AltTitlesResult struct {
	Source string   `json:"source"`
	Titles []string `json:"titles"`
}

// AltTitlesProvider returns the plugin id that exposes GetAltTitlesFunc, or "".
// The host picks whichever plugin declares the capability — provider selection
// lives in plugin metadata, not in host code.
func (m *Manager) AltTitlesProvider() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for id := range m.plugins {
		if m.pluginHasFunc(id, types.GetAltTitlesFunc) {
			return id
		}
	}
	return ""
}

// GetAltTitles calls the provider plugin to resolve alternative titles.
func (m *Manager) GetAltTitles(title string) (AltTitlesResult, error) {
	provider := m.AltTitlesProvider()
	if provider == "" {
		return AltTitlesResult{}, errors.New("no alt-titles provider plugin")
	}
	p, err := m.get(provider)
	if err != nil {
		return AltTitlesResult{}, err
	}
	out, err := m.call(p, types.GetAltTitlesFunc, `"`+strings.ReplaceAll(title, `"`, `\"`)+`"`)
	if err != nil {
		return AltTitlesResult{}, err
	}
	var res AltTitlesResult
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		return AltTitlesResult{}, fmt.Errorf("alt-titles decode: %w", err)
	}
	if res.Source == "" {
		res.Source = provider
	}
	return res, nil
}

// pluginHasFunc reports whether a plugin's runtime exposes the given ABI
// function. It never triggers lazy instantiation for deferred plugins; an
// unloaded plugin is checked by looking at its declared runtime later — for
// now we conservatively load it (alt-titles is a rare, user-triggered path).
func (m *Manager) pluginHasFunc(id, fnName string) bool {
	p, err := m.get(id)
	if err != nil {
		return false
	}
	switch p.kind {
	case "js":
		jsName, ok := jsFnNames[fnName]
		if !ok {
			return false
		}
		if err := m.ensureLoaded(id); err != nil {
			return false
		}
		v := p.js.Get(jsName)
		return v != nil && !goja.IsUndefined(v)
	}
	return false
}

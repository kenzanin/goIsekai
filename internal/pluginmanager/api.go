package pluginmanager

import (
	"github.com/goccy/go-json"
	"fmt"
	"goisekai/internal/logger"


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

// GetMangaDetail runs a plugin's GetMangaDetail function and decodes its result.
// It uses a cached response if available and not expired.
func (m *Manager) GetMangaDetail(pluginID, mangaID string) (types.Manga, error) {
	p, err := m.get(pluginID)
	if err != nil {
		return types.Manga{}, err
	}

	// Check cache first for GetMangaDetail responses.
	if m.db != nil {
		if cached, err := m.db.GetCache(pluginID, mangaID, types.GetMangaDetailFunc); err == nil {
			var result types.Manga
			if err := json.Unmarshal([]byte(cached), &result); err == nil {
				m.db.RecordCacheHit()
				return result, nil
			}
		}
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
	// Cache the result for future calls.
	if m.db != nil {
		if cacheErr := m.db.SetCache(pluginID, mangaID, types.GetMangaDetailFunc, out, m.cacheTTL); cacheErr != nil {
			logger.Warn("cache set", "plugin", pluginID, "manga", mangaID, "error", cacheErr)
		}
	}
	return result, nil
}

// GetChapterList runs a plugin's GetChapterList function and decodes its result.
// It uses a cached response if available and not expired.
func (m *Manager) GetChapterList(pluginID, mangaID string) ([]types.Chapter, error) {
	p, err := m.get(pluginID)
	if err != nil {
		return nil, err
	}

	// Check cache first for GetChapterList responses.
	if m.db != nil {
		if cached, err := m.db.GetCache(pluginID, mangaID, types.GetChapterListFunc); err == nil {
			var result []types.Chapter
			if err := json.Unmarshal([]byte(cached), &result); err == nil {
				m.db.RecordCacheHit()
				return result, nil
			}
		}
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
	// Cache the result for future calls.
	if m.db != nil {
		if cacheErr := m.db.SetCache(pluginID, mangaID, types.GetChapterListFunc, out, m.cacheTTL); cacheErr != nil {
			logger.Warn("cache set", "plugin", pluginID, "manga", mangaID, "error", cacheErr)
		}
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

// GetMangaDetailWithChapters runs a plugins GetMangaDetailWithChapters batch function
// if available (fallback to separate GetMangaDetail + GetChapterList calls).
func (m *Manager) GetMangaDetailWithChapters(pluginID, mangaID string) (types.Manga, []types.Chapter, error) {
	p, err := m.get(pluginID)
	if err != nil {
		return types.Manga{}, nil, err
	}

	// Try batch function first.
	in, err := json.Marshal(mangaID)
	if err != nil {
		return types.Manga{}, nil, err
	}
	out, err := m.call(p, types.GetMangaDetailWithChaptersFunc, string(in))
	if err == nil {
		// Parse the batch response: {"manga": Manga, "chapters": [Chapter]}
		var batchResp struct {
			Manga     types.Manga   `json:"manga"`
			Chapters  []types.Chapter `json:"chapters"`
		}
		if err := json.Unmarshal([]byte(out), &batchResp); err == nil {
			return batchResp.Manga, batchResp.Chapters, nil
		}
	}
	// Fallback to separate calls.
	manga, err := m.GetMangaDetail(pluginID, mangaID)
	if err != nil {
		return types.Manga{}, nil, err
	}
	chapters, err := m.GetChapterList(pluginID, mangaID)
	if err != nil {
		return types.Manga{}, nil, err
	}
	return manga, chapters, nil
}

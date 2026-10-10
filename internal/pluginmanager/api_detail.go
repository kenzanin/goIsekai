package pluginmanager

import (
	"fmt"

	"github.com/goccy/go-json"

	"goisekai/internal/logger"
	"goisekai/pkg/types"
)

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
	// Cache the result for future calls — but never cache an empty detail:
	// a transient upstream failure (rate limit, WAF) makes plugins return a
	// bare {id} with nil error, and caching that would blank the detail page
	// until the TTL expires. Leave the cache absent so the next call retries.
	if m.db != nil && result.Title != "" {
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
	// Same empty-guard as GetMangaDetail: plugins return [] on transient
	// upstream failures — never poison the cache with an empty list.
	if m.db != nil && len(result) > 0 {
		if cacheErr := m.db.SetCache(pluginID, mangaID, types.GetChapterListFunc, out, m.chapterCacheTTL); cacheErr != nil {
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
			Manga    types.Manga     `json:"manga"`
			Chapters []types.Chapter `json:"chapters"`
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

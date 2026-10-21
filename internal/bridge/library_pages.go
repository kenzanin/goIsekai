package bridge

import (
	"context"
	"fmt"

	"goisekai/internal/logger"
	"goisekai/pkg/types"

	"github.com/goccy/go-json"
)

// resolveChapterIntID looks up the integer chapter ID from source identifiers.
func (s *AppService) resolveChapterIntID(pluginID, mangaID, chapterID string) (int64, error) {
	return s.db.ResolveChapterIntID(pluginID, mangaID, chapterID)
}

// GetPageList delegates to the plugin's GetPageList function.
func (s *AppService) GetPageList(pluginID, chapterID string) ([]types.Page, error) {
	// Deactivated plugins must not issue remote requests.
	if !s.db.IsPluginActive(pluginID) {
		chapterIntID, _ := s.resolveChapterIntID(pluginID, "", chapterID)
		cached, err := s.db.GetChapterPages(chapterIntID)
		if err != nil || cached == nil {
			return nil, fmt.Errorf("bridge: get page list: plugin %s is offline and has no cached pages", pluginID)
		}
		var pages []types.Page
		if uerr := json.Unmarshal(cached, &pages); uerr != nil {
			return nil, fmt.Errorf("bridge: unmarshal cached pages: %w", uerr)
		}
		return pages, nil
	}

	var (
		result []types.Page
		err    error
	)
	runErr := s.runOnInteractive(context.Background(), pluginID, func(context.Context) error {
		result, err = s.mgr.GetPageList(pluginID, chapterID)
		return err
	})
	if runErr != nil {
		return nil, fmt.Errorf("bridge: get page list: %w", runErr)
	}
	if err != nil {
		return nil, fmt.Errorf("bridge: get page list: %w", err)
	}
	if raw, merr := json.Marshal(result); merr == nil {
		chapterIntID, _ := s.resolveChapterIntID(pluginID, "", chapterID)
		if chapterIntID != 0 {
			if perr := s.db.SaveChapterPages(chapterIntID, raw); perr != nil {
				logger.Warn("cache chapter pages", "chapter", chapterID, "error", perr)
			}
		}
	}
	return result, nil
}

// GetPageListCached returns the cached page list from the database immediately
// without blocking on the interactive lane. Returns nil pages for empty result
// (plugin returned no pages) so caller can handle gracefully.
func (s *AppService) GetPageListCached(pluginID, chapterID string) ([]types.Page, error) {
	chapterIntID, _ := s.resolveChapterIntID(pluginID, "", chapterID)
	if chapterIntID == 0 {
		return nil, nil // chapter not found in DB - will fallback to live fetch
	}
	cached, err := s.db.GetChapterPages(chapterIntID)
	if err != nil {
		return nil, fmt.Errorf("bridge: get chapter pages: %w", err)
	}
	if cached == nil {
		return nil, nil // no cached pages - will fallback to live fetch
	}
	var pages []types.Page
	if uerr := json.Unmarshal(cached, &pages); uerr != nil {
		return nil, fmt.Errorf("bridge: unmarshal cached pages: %w", uerr)
	}
	return pages, nil
}

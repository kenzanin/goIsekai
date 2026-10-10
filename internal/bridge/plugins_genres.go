package bridge

import (
	"encoding/json"
	"fmt"

	"goisekai/internal/logger"
	"goisekai/internal/pluginmanager"
)

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

package bridge

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"goisekai/internal/database"
	"goisekai/internal/logger"
	"goisekai/internal/pluginmanager"
)

// RefreshSummary reports what a plugin resync changed.
type RefreshSummary struct {
	Added       int `json:"added"`
	Deactivated int `json:"deactivated"`
	Updated     int `json:"updated"`
	Purged      int `json:"purged"`
}

// RefreshPlugins re-syncs the plugin table with the plugins directory.
// Files added since startup are registered, rows whose files are gone are
// deactivated (history references plugin_id, so rows are never deleted),
// backup artifacts (*.bak.*) are purged, and metadata is refreshed.
func (s *AppService) RefreshPlugins() (RefreshSummary, error) {
	var sum RefreshSummary
	// Pick up files added since startup. Dups are skipped by discovery.
	if err := s.mgr.Discover(); err != nil {
		return sum, fmt.Errorf("bridge: rediscover plugins: %w", err)
	}
	loaded := map[string]pluginmanager.LoadedPlugin{}
	for _, p := range s.mgr.LoadedPlugins() {
		loaded[p.ID] = p
	}
	dbPlugins, err := s.db.ListPlugins()
	if err != nil {
		return sum, fmt.Errorf("bridge: list plugins: %w", err)
	}
	dbByID := map[string]database.Plugin{}
	for _, p := range dbPlugins {
		dbByID[p.ID] = p
	}
	// Purge backup artifacts first: they were never real plugins.
	for id := range dbByID {
		if strings.Contains(id, ".bak.") {
			if err := s.db.DeletePlugin(id); err != nil {
				logger.Warn("refresh: purge bak failed", "id", id, "error", err)
				continue
			}
			sum.Purged++
			delete(dbByID, id)
		}
	}
	// Register new files.
	for id, p := range loaded {
		if _, ok := dbByID[id]; !ok {
			if err := s.db.RegisterPlugin(database.Plugin{
				ID:       id,
				Name:     id,
				Version:  p.Version,
				WasmPath: p.WasmPath,
				IsActive: true,
			}); err != nil {
				logger.Warn("refresh: register failed", "id", id, "error", err)
				continue
			}
			sum.Added++
		}
	}
	// Deactivate rows whose files are gone; refresh metadata for the rest.
	for id, dbp := range dbByID {
		lp, ok := loaded[id]
		if !ok {
			// Not in memory. Check disk before deactivating: the manager
			// may simply not have loaded it yet.
			if _, err := os.Stat(dbp.WasmPath); os.IsNotExist(err) {
				if dbp.IsActive {
					if err := s.db.SetPluginActive(id, false); err != nil {
						logger.Warn("refresh: deactivate failed", "id", id, "error", err)
						continue
					}
					sum.Deactivated++
				}
			}
			continue
		}
		// Refresh metadata if the files moved or the version changed.
		if dbp.WasmPath != lp.WasmPath || dbp.Version != lp.Version {
			if err := s.db.RegisterPlugin(database.Plugin{
				ID:       id,
				Name:     dbp.Name,
				Version:  lp.Version,
				WasmPath: lp.WasmPath,
				IsActive: dbp.IsActive,
			}); err != nil {
				logger.Warn("refresh: update failed", "id", id, "error", err)
				continue
			}
			sum.Updated++
		}
	}
	return sum, nil
}

// PurgeRemovedPlugins deletes every trace of plugins whose files are gone from
// disk: their manga rows (chapters, history and enrichment data cascade),
// response cache, verify cookies, FTS documents, the plugins row itself, and
// their image cache directory. Then it VACUUMs once to reclaim the space.
//
// This is the destructive counterpart to RefreshPlugins, which only flips such
// rows to is_active=0 because reading history still references plugin_id.
// Returns a human-readable summary; an empty result means nothing was removed.
func (s *AppService) PurgeRemovedPlugins() (string, error) {
	dbPlugins, err := s.db.ListPlugins()
	if err != nil {
		return "", fmt.Errorf("bridge: list plugins: %w", err)
	}
	// Pass 1 — decide eligibility before touching anything. wasm_path is stored
	// relative to the plugin root, so a server started from the wrong working
	// directory reports EVERY plugin as missing; purge has to refuse in that
	// case instead of wiping the whole library.
	var removed []database.Plugin
	present := 0
	for _, p := range dbPlugins {
		if _, err := os.Stat(p.WasmPath); !os.IsNotExist(err) {
			present++
			continue
		}
		removed = append(removed, p)
	}
	if len(removed) == 0 {
		return "clean", nil
	}
	if present == 0 {
		return "", fmt.Errorf("refusing to purge: no plugin files found at all (wrong working directory?)")
	}

	// Pass 2 — delete.
	var b strings.Builder
	for _, p := range removed {
		summary, err := s.db.PurgeRemovedPlugin(p.ID)
		if err != nil {
			return b.String(), fmt.Errorf("bridge: purge %s: %w", p.ID, err)
		}
		if s.cacheDir != "" {
			if err := os.RemoveAll(filepath.Join(s.cacheDir, "images", p.ID)); err != nil {
				logger.Warn("purge: image cache remove failed", "id", p.ID, "error", err)
			}
		}
		fmt.Fprintf(&b, "%s: %s; ", p.ID, summary)
	}
	if err := s.db.Vacuum(); err != nil {
		logger.Warn("purge: vacuum failed", "error", err)
	}
	logger.Debug("purged removed plugins", "count", len(removed))
	return strings.TrimSpace(strings.TrimSuffix(b.String(), "; ")), nil
}

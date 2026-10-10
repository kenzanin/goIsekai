package bridge

import (
	"fmt"
	"os"
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

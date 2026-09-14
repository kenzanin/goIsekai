package database

import (
	"sync/atomic"
	"fmt"
	"time"

	"goisekai/internal/logger"
)

// cacheKey returns the deterministic cache row key: "pluginID|mangaID|funcName".
func cacheKey(pluginID, mangaID, funcName string) string {
	return pluginID + "\x00" + mangaID + "\x00" + funcName
}

// GetCache returns the cached response for a plugin function call, or sql.ErrNoRows.
func (d *DB) GetCache(pluginID, mangaID, funcName string) (string, error) {
	var response string
	err := d.db.QueryRow(
		`SELECT response FROM plugin_cache
		 WHERE id = ? AND expires_at > datetime('now')`,
		cacheKey(pluginID, mangaID, funcName),
	).Scan(&response)
	if err != nil {
		return "", err
	}
	return response, nil
}

// SetCache stores a JSON response with the given TTL.
func (d *DB) SetCache(pluginID, mangaID, funcName, response string, ttl time.Duration) error {
	_, err := d.db.Exec(
		`INSERT OR REPLACE INTO plugin_cache (id, plugin_id, manga_id, function_name, response, cached_at, expires_at)
		 VALUES (?, ?, ?, ?, ?, datetime('now'), datetime('now', ?))`,
		cacheKey(pluginID, mangaID, funcName),
		pluginID, mangaID, funcName, response,
		fmt.Sprintf("+%d hours", int(ttl.Hours())),
	)
	if err != nil {
		return fmt.Errorf("cache set: %w", err)
	}
	return nil
}

// DeleteCache removes all cached entries for a specific manga.
func (d *DB) DeleteCache(pluginID, mangaID string) error {
	_, err := d.db.Exec(
		`DELETE FROM plugin_cache WHERE plugin_id = ? AND manga_id = ?`,
		pluginID, mangaID,
	)
	if err != nil {
		return fmt.Errorf("cache delete: %w", err)
	}
	return nil
}

// DeleteCacheByPlugin removes all cached entries for a plugin.
func (d *DB) DeleteCacheByPlugin(pluginID string) error {
	_, err := d.db.Exec(
		`DELETE FROM plugin_cache WHERE plugin_id = ?`,
		pluginID,
	)
	if err != nil {
		return fmt.Errorf("cache delete by plugin: %w", err)
	}
	return nil
}

// CleanExpired removes entries whose TTL has passed. Returns count deleted.
func (d *DB) CleanExpired() (int64, error) {
	result, err := d.db.Exec(`DELETE FROM plugin_cache WHERE expires_at <= datetime('now')`)
	if err != nil {
		return 0, fmt.Errorf("cache clean: %w", err)
	}
	return result.RowsAffected()
}

// CacheCount returns the total number of entries in the cache.
func (d *DB) CacheCount() (int64, error) {
	var count int64
	err := d.db.QueryRow(`SELECT COUNT(*) FROM plugin_cache`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("cache count: %w", err)
	}
	return count, nil
}

// CacheHitCount returns the total number of successful cache lookups.
var cacheHits int64

// RecordCacheHit increments the cache hit counter.
func (d *DB) RecordCacheHit() {
	atomic.AddInt64(&cacheHits, 1)
	// ponytail: use sync/atomic when the DB struct is thread-safe for counters.
	// For now, this is a simple counter for /api/stats.
}

// CacheStats returns cache metrics for the stats endpoint.
func (d *DB) CacheStats() (total, hits int64, err error) {
	total, err = d.CacheCount()
	if err != nil {
		return 0, 0, err
	}
	return total, cacheHits, nil
}

// StartCacheCleanup starts a background goroutine that cleans expired cache
// entries every hour. It stops when the cleanup channel is closed.
func (d *DB) StartCacheCleanup(maintenanceStop <-chan struct{}) {
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-maintenanceStop:
				return
			case <-ticker.C:
				n, err := d.CleanExpired()
				if err != nil {
					logger.Error("cache cleanup", "error", err)
				} else if n > 0 {
					logger.Info("cleaned expired cache entries", "count", n)
				}
			}
		}
	}()
}

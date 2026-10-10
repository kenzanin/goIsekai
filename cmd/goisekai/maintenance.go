package main

import (
	"context"
	"math"
	"path/filepath"
	"sync/atomic"
	"time"

	"goisekai/internal/config"
	"goisekai/internal/database"
	"goisekai/internal/hostnet"
	"goisekai/internal/logger"
	"goisekai/internal/pluginmanager"
	"goisekai/internal/workers"
)

// startConfigWatch polls goisekai.ini and applies the safe subset (log level,
// user-agent, referer, max_cache_gb) live. Unsafe fields like
// host/port/cdp_engine need a restart, so they are deliberately not applied here.
func startConfigWatch(cfgPath string, proxy *hostnet.Proxy, maxCacheGB *atomic.Int64) {
	_ = config.Watch(cfgPath, 5*time.Second, func(updated *config.Config) {
		if err := logger.Init(updated.LogLevel); err == nil {
			logger.Info("config reloaded", "log_level", updated.LogLevel)
		}
		proxy.SetDefaultHeader("User-Agent", updated.UserAgent)
		proxy.SetDefaultHeader("Referer", updated.Referer)
		proxy.SetSecCHUA(updated.SecCHUA)
		maxCacheGB.Store(int64(math.Float64bits(updated.MaxCacheGB)))
		logger.Info("config reloaded", "log_level", updated.LogLevel, "max_cache_gb", updated.MaxCacheGB)
	})
}

// startMaintenance prunes orphaned rows at startup, then backs up + re-prunes
// on the configured interval until the returned channel is closed.
// maxCacheGB holds the hot-reloadable image-cache size cap (float64 bits).
// The pool handles the periodic maintenance jobs; a stop channel is returned
// for graceful shutdown. Initial work runs immediately.
func startMaintenance(ctx context.Context, db *database.DB, cfg *config.Config, dataDir string, maxCacheGB *atomic.Int64, pool *workers.Pool) chan struct{} {
	backupsDir := filepath.Join(dataDir, "backups")
	healFTS := func() {
		rebuilt, err := db.EnsureLibraryFTS()
		if err != nil {
			logger.Error("library fts check", "error", err)
			return
		}
		if rebuilt {
			logger.Info("library_fts index rebuilt")
		}
	}
	if cfg.PruneOrphans {
		if summary, err := db.PruneOrphans(); err != nil {
			logger.Error("prune orphans", "error", err)
		} else if summary != "clean" {
			logger.Info("pruned orphaned rows", "summary", summary)
		}
	}
	// Keep the library search index consistent with library membership.
	healFTS()
	// Prune image cache at startup based on MaxCacheGB config.
	if _, err := pruneImageCache(cfg, dataDir, maxCacheGBBits(maxCacheGB)); err != nil {
		logger.Error("prune image cache at startup", "error", err)
	}

	// If backup interval is configured, do an initial backup at startup.
	if cfg.BackupIntervalHours > 0 {
		if _, err := db.BackupTo(backupsDir, cfg.BackupKeep); err != nil {
			logger.Error("db backup", "error", err)
		} else {
			logger.Info("db backup written", "dir", backupsDir, "keep", cfg.BackupKeep)
		}
	}

	stop := make(chan struct{})
	if cfg.BackupIntervalHours <= 0 {
		return stop // backups disabled, no background work
	}

	// Batch all maintenance tasks into one job for the maintenance lane.
	doBatch := func(ctx context.Context) error {
		if cfg.PruneOrphans {
			if summary, err := db.PruneOrphans(); err == nil && summary != "clean" {
				logger.Info("pruned orphaned rows", "summary", summary)
			}
		}
		healFTS()
		if _, err := db.BackupTo(backupsDir, cfg.BackupKeep); err != nil {
			logger.Error("db backup", "error", err)
		} else {
			logger.Info("db backup written", "dir", backupsDir, "keep", cfg.BackupKeep)
		}
		if _, err := pruneImageCache(cfg, dataDir, maxCacheGBBits(maxCacheGB)); err != nil {
			logger.Error("prune image cache", "error", err)
		}
		return nil
	}

	go func() {
		interval := time.Duration(cfg.BackupIntervalHours) * time.Hour
		backupJob := &workers.Job{
			Lane: workers.LaneMaintenance,
			Run:  func(ctx context.Context) error { return doBatch(ctx) },
		}
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				if _, err := pool.Enqueue(ctx, backupJob); err != nil {
					logger.Error("maintenance enqueue", "error", err)
				}
			}
		}
	}()
	return stop
}

// preconnectHosts warms the connection pool for every known plugin host.
func preconnectHosts(mgr *pluginmanager.Manager) {
	hosts := mgr.GetKnownHosts()
	logger.Info("preconnecting to known hosts", "count", len(hosts))
	sem := make(chan struct{}, 4) // concurrency limit 4
	for _, host := range hosts {
		sem <- struct{}{}
		go func(h string) {
			defer func() { <-sem }()
			mgr.Proxy().Preconnect(h)
		}(host)
	}
}

// registerLoadedPlugins persists plugins loaded from the plugins dir so they
// appear in ListPlugins (Discover only loads them into memory).
func registerLoadedPlugins(db *database.DB, mgr *pluginmanager.Manager) {
	for _, p := range mgr.LoadedPlugins() {
		if err := db.RegisterPlugin(database.Plugin{
			ID:         p.ID,
			Name:       p.ID,
			Version:    p.Version,
			WasmPath:   p.WasmPath,
			IsActive:   true,
			ThumbRatio: p.ThumbRatio,
		}); err != nil {
			logger.Error("register discovered plugin", "id", p.ID, "error", err)
		}
	}
}

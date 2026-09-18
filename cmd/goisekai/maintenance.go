package main

import (
	"path/filepath"
	"time"

	"goisekai/internal/config"
	"goisekai/internal/database"
	"goisekai/internal/hostnet"
	"goisekai/internal/logger"
	"goisekai/internal/pluginmanager"
)

// startConfigWatch polls goisekai.ini and applies the safe subset (log level,
// user-agent, referer) live. Unsafe fields like host/port/cdp_engine need a
// restart, so they are deliberately not applied here.
func startConfigWatch(cfgPath string, proxy *hostnet.Proxy) {
	_ = config.Watch(cfgPath, 5*time.Second, func(updated *config.Config) {
		if err := logger.Init(updated.LogLevel); err == nil {
			logger.Info("config reloaded", "log_level", updated.LogLevel)
		}
		proxy.SetDefaultHeader("User-Agent", updated.UserAgent)
		proxy.SetDefaultHeader("Referer", updated.Referer)
		proxy.SetSecCHUA(updated.SecCHUA)
	})
}

// startMaintenance prunes orphaned rows at startup, then backs up + re-prunes
// on the configured interval until the returned channel is closed.
func startMaintenance(db *database.DB, cfg *config.Config, dataDir string) chan struct{} {
	backupsDir := filepath.Join(dataDir, "backups")
	if cfg.PruneOrphans {
		if summary, err := db.PruneOrphans(); err != nil {
			logger.Error("prune orphans", "error", err)
		} else if summary != "clean" {
			logger.Info("pruned orphaned rows", "summary", summary)
		}
	}
	stop := make(chan struct{})
	go func() {
		interval := time.Duration(cfg.BackupIntervalHours) * time.Hour
		if interval <= 0 {
			return // backups disabled
		}
		backup := func() {
			if _, err := db.BackupTo(backupsDir, cfg.BackupKeep); err != nil {
				logger.Error("db backup", "error", err)
			} else {
				logger.Info("db backup written", "dir", backupsDir, "keep", cfg.BackupKeep)
			}
		}
		backup() // first backup at startup
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				if cfg.PruneOrphans {
					if summary, err := db.PruneOrphans(); err == nil && summary != "clean" {
						logger.Info("pruned orphaned rows", "summary", summary)
					}
				}
				backup()
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

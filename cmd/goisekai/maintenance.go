package main

import (
	"math"
	"os"
	"path/filepath"
	"sort"
	"sync/atomic"
	"time"

	"goisekai/internal/config"
	"goisekai/internal/database"
	"goisekai/internal/hostnet"
	"goisekai/internal/logger"
	"goisekai/internal/pluginmanager"
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

// maxCacheGBBits reads the atomically-stored MaxCacheGB value.
func maxCacheGBBits(v *atomic.Int64) float64 { return math.Float64frombits(uint64(v.Load())) }

// pruneImageCache walks the image cache directory and deletes the oldest files
// until the total size is at or below maxBytesGB (in GB). Returns bytes freed.
func pruneImageCache(cfg *config.Config, dataDir string, maxBytesGB float64) (int64, error) {
	if maxBytesGB <= 0 {
		return 0, nil
	}
	cacheDir := cfg.CacheDir
	if cacheDir == "" {
		cacheDir = filepath.Join(dataDir, "cache")
	}
	dir := filepath.Join(cacheDir, "images")
	type fileEntry struct {
		path  string
		size  int64
		mtime time.Time
	}
	var files []fileEntry
	var total int64
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if !info.IsDir() {
			total += info.Size()
			files = append(files, fileEntry{path: path, size: info.Size(), mtime: info.ModTime()})
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	target := int64(maxBytesGB * 1024 * 1024 * 1024)
	if total <= target {
		return 0, nil
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].mtime.Before(files[j].mtime)
	})
	var freed int64
	for _, f := range files {
		if total <= target {
			break
		}
		if err := os.Remove(f.path); err != nil {
			logger.Warn("prune cache delete", "file", f.path, "error", err)
			continue
		}
		total -= f.size
		freed += f.size
	}
	logger.Info("pruned image cache", "freed_bytes", freed)
	return freed, nil
}

// startMaintenance prunes orphaned rows at startup, then backs up + re-prunes
// on the configured interval until the returned channel is closed.
// maxCacheGB holds the hot-reloadable image-cache size cap (float64 bits).
func startMaintenance(db *database.DB, cfg *config.Config, dataDir string, maxCacheGB *atomic.Int64) chan struct{} {
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
				healFTS()
				backup()
				// Re-read MaxCacheGB each tick (hot-reloadable) and prune if needed.
				if _, err := pruneImageCache(cfg, dataDir, maxCacheGBBits(maxCacheGB)); err != nil {
					logger.Error("prune image cache", "error", err)
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

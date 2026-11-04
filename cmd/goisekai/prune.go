package main

import (
	"math"
	"os"
	"path/filepath"
	"sort"
	"sync/atomic"
	"time"

	"goisekai/internal/config"
	"goisekai/internal/logger"
)

// maxCacheGBBits reads the atomically-stored MaxCacheGB value.
func maxCacheGBBits(v *atomic.Int64) float64 { return math.Float64frombits(uint64(v.Load())) }

// pruneImageCache walks the image cache directory and deletes files until the
// total size is at or below maxBytesGB. Unprotected files go first, oldest
// mtime first; protected files (library manga covers and chapter images,
// decided by `protected`) are deleted by mtime only if the cap is still
// exceeded after the unprotected sweep. Returns bytes freed.
func pruneImageCache(cfg *config.Config, dataDir string, maxBytesGB float64, protected func(path string) bool) (int64, error) {
	if maxBytesGB <= 0 {
		return 0, nil
	}
	cacheDir := cfg.CacheDir
	if cacheDir == "" {
		cacheDir = filepath.Join(dataDir, "cache")
	}
	dir := filepath.Join(cacheDir, "images")
	type fileEntry struct {
		path    string
		size    int64
		mtime   time.Time
		protect bool
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
			files = append(files, fileEntry{path: path, size: info.Size(), mtime: info.ModTime(), protect: protected != nil && protected(path)})
			total += info.Size()
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
	// ponytail: one sort with protect as the dominant key covers both the
	// "unprotected first" and "protected only if still over" requirements, so
	// the log below can report per-group freed bytes.
	sort.Slice(files, func(i, j int) bool {
		// Unprotected entries come first, then oldest-first within each group.
		if files[i].protect != files[j].protect {
			return !files[i].protect
		}
		return files[i].mtime.Before(files[j].mtime)
	})
	var freed, freedUnprotected, freedProtected int64
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
		if f.protect {
			freedProtected += f.size
		} else {
			freedUnprotected += f.size
		}
		logger.Debug("prune cache file", "file", f.path, "protected", f.protect, "bytes", f.size)
	}
	logger.Info("pruned image cache",
		"freed_bytes", freed,
		"freed_unprotected_bytes", freedUnprotected,
		"freed_protected_bytes", freedProtected)
	return freed, nil
}

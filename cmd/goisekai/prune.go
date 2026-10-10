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

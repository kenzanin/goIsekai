package bridge

import (
	"fmt"
	"goisekai/internal/logger"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// ClearMangaCache removes every cached image file for one manga. It frees disk
// only; the in-memory L1 cache is left untouched (bounded and short-lived).
func (s *AppService) ClearMangaCache(pluginID, mangaID string) error {
	if s.cacheDir == "" {
		return nil
	}
	dir := filepath.Join(s.cacheDir, "images", pluginID, mangaID)
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("bridge: clear manga cache: %w", err)
	}
	logger.Info("cleared manga cache", "plugin", pluginID, "manga", mangaID)
	return nil
}

// ClearAllCache removes the entire image cache directory and drops the L1 map.
func (s *AppService) ClearAllCache() error {
	if s.cacheDir == "" {
		return nil
	}
	dir := filepath.Join(s.cacheDir, "images")
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("bridge: clear all cache: %w", err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("bridge: clear all cache: mkdir: %w", err)
	}
	s.imageMu.Lock()
	s.imageCache = make(map[string][]byte)
	s.imageMu.Unlock()
	logger.Info("cleared all image cache")
	return nil
}

// CacheSize returns the total bytes of all cached image files on disk.
func (s *AppService) CacheSize() (int64, error) {
	if s.cacheDir == "" {
		return 0, nil
	}
	dir := filepath.Join(s.cacheDir, "images")
	var total int64
	err := filepath.Walk(dir, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if !info.IsDir() {
			total += info.Size()
		}
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("bridge: cache size: %w", err)
	}
	return total, nil
}

// PruneImageCache walks the image cache directory and deletes the oldest files
// until the total size is at or below maxBytes. Returns the bytes freed.
// No-op when maxBytes <= 0 or cacheDir is empty.
func (s *AppService) PruneImageCache(maxBytes int64) (int64, error) {
	if maxBytes <= 0 || s.cacheDir == "" {
		return 0, nil
	}
	dir := filepath.Join(s.cacheDir, "images")
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
		return 0, fmt.Errorf("bridge: prune cache walk: %w", err)
	}
	if total <= maxBytes {
		return 0, nil
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].mtime.Before(files[j].mtime)
	})
	var freed int64
	for _, f := range files {
		if total <= maxBytes {
			break
		}
		if err := os.Remove(f.path); err != nil {
			logger.Warn("prune cache delete", "file", f.path, "error", err)
			continue
		}
		total -= f.size
		freed += f.size
	}
	logger.Info("pruned image cache", "freed_bytes", freed, "remaining_bytes", total)
	return freed, nil
}

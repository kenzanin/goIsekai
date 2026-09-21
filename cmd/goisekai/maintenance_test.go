package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"goisekai/internal/config"
)

func TestPruneImageCache(t *testing.T) {
	// Create temp cache directory with test files.
	tmpDir := t.TempDir()
	cacheDir := filepath.Join(tmpDir, "cache", "images")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatalf("mkdir cache: %v", err)
	}

	// Create 3 files with different mtimes.
	now := time.Now()
	oldest := filepath.Join(cacheDir, "oldest.jpg")
	middle := filepath.Join(cacheDir, "middle.jpg")
	newest := filepath.Join(cacheDir, "newest.jpg")

	files := []string{oldest, middle, newest}
	for i, path := range files {
		data := []byte("test content " + string(rune('A'+i)))
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatalf("write file: %v", err)
		}
	}

	// Set mtimes: oldest 2 days ago, middle 1 day ago, newest now.
	if err := os.Chtimes(oldest, now.Add(-48*time.Hour), now.Add(-48*time.Hour)); err != nil {
		t.Fatalf("chtime oldest: %v", err)
	}
	if err := os.Chtimes(middle, now.Add(-24*time.Hour), now.Add(-24*time.Hour)); err != nil {
		t.Fatalf("chtime middle: %v", err)
	}
	// newest keeps current time.

	// Create config with small max_cache_gb (100KB).
	cfg := &config.Config{
		CacheDir: filepath.Join(tmpDir, "cache"),
	}

	// Calculate file sizes and set target limit to keep newest two.
	oldestSize, _ := os.Stat(oldest)
	middleSize, _ := os.Stat(middle)
	newestSize, _ := os.Stat(newest)
	totalSize := oldestSize.Size() + middleSize.Size() + newestSize.Size()

	// Set limit to keep only the newest file (total - oldest).
	limitGB := float64(totalSize-oldestSize.Size()) / (1024 * 1024 * 1024)

	freed, err := pruneImageCache(cfg, tmpDir, limitGB)
	if err != nil {
		t.Fatalf("pruneImageCache: %v", err)
	}

	// Verify oldest is deleted.
	if _, err := os.Stat(oldest); !os.IsNotExist(err) {
		t.Errorf("oldest file should be deleted, but exists")
	}

	// Verify middle and newest still exist.
	if _, err := os.Stat(middle); os.IsNotExist(err) {
		t.Errorf("middle file should exist")
	}
	if _, err := os.Stat(newest); os.IsNotExist(err) {
		t.Errorf("newest file should exist")
	}

	// Verify freed amount.
	if freed != oldestSize.Size() {
		t.Errorf("freed = %d, want %d", freed, oldestSize.Size())
	}

	_ = middleSize // avoid unused variable warning
	_ = newestSize // avoid unused variable warning
}

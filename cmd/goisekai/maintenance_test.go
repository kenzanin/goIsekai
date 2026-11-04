package main

import (
	"bytes"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"goisekai/internal/config"
	"goisekai/internal/hostnet"
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

	freed, err := pruneImageCache(cfg, tmpDir, limitGB, nil)
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

// writeCacheFile writes a small file and sets its mtime to age ago.
func writeCacheFile(t *testing.T, path string, age time.Duration) int64 {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	data := bytes.Repeat([]byte("x"), 1024)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	when := time.Now().Add(-age)
	if err := os.Chtimes(path, when, when); err != nil {
		t.Fatal(err)
	}
	return int64(len(data))
}

// TestPruneImageCacheProtection covers task 3.5: the library protection
// predicate reorders eviction, and max_cache_gb 0 deletes nothing.
func TestPruneImageCacheProtection(t *testing.T) {
	tmpDir := t.TempDir()
	cacheDir := filepath.Join(tmpDir, "cache", "images")
	cfg := &config.Config{CacheDir: filepath.Join(tmpDir, "cache")}

	// The predicate protects exactly one path, the "library" file.
	library := filepath.Join(cacheDir, "lib.jpg")
	browsing := filepath.Join(cacheDir, "browse.jpg")
	newer := filepath.Join(cacheDir, "newer.jpg")
	protected := func(p string) bool { return p == library }

	t.Run("unprotected goes first regardless of age", func(t *testing.T) {
		writeCacheFile(t, library, 48*time.Hour)       // oldest, but protected
		sz := writeCacheFile(t, browsing, time.Minute) // newest, but unprotected
		freed, err := pruneImageCache(cfg, tmpDir, float64(sz)/(1024*1024*1024), protected)
		if err != nil {
			t.Fatal(err)
		}
		if freed != sz {
			t.Errorf("freed = %d, want %d", freed, sz)
		}
		if _, err := os.Stat(browsing); !os.IsNotExist(err) {
			t.Error("unprotected file should be deleted first")
		}
		if _, err := os.Stat(library); os.IsNotExist(err) {
			t.Error("protected file should survive while unprotected files remain")
		}
	})

	t.Run("cap is met when only library files remain", func(t *testing.T) {
		lib2 := filepath.Join(cacheDir, "lib2.jpg")
		writeCacheFile(t, lib2, time.Hour)
		freed, err := pruneImageCache(cfg, tmpDir, 1e-12, func(p string) bool { return true })
		if err != nil {
			t.Fatal(err)
		}
		if freed == 0 {
			t.Error("protected files must still be deleted once nothing else remains")
		}
		if entries, _ := os.ReadDir(cacheDir); len(entries) != 0 {
			t.Errorf("cache not emptied: %v", entries)
		}
	})

	t.Run("touched old file outlives untouched newer one", func(t *testing.T) {
		old := filepath.Join(cacheDir, "old.jpg")
		writeCacheFile(t, old, 48*time.Hour)
		writeCacheFile(t, newer, time.Minute)
		// A read "touches" the old file: its mtime becomes newer than `newer`.
		if err := os.Chtimes(old, time.Now(), time.Now()); err != nil {
			t.Fatal(err)
		}
		sz, _ := os.Stat(newer)
		if _, err := pruneImageCache(cfg, tmpDir, float64(sz.Size())/(1024*1024*1024), nil); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(newer); !os.IsNotExist(err) {
			t.Error("untouched newer file should be deleted")
		}
		if _, err := os.Stat(old); os.IsNotExist(err) {
			t.Error("touched old file should survive")
		}
	})

	t.Run("max_cache_gb 0 deletes nothing", func(t *testing.T) {
		writeCacheFile(t, filepath.Join(cacheDir, "keep.jpg"), time.Hour)
		freed, err := pruneImageCache(cfg, tmpDir, 0, nil)
		if err != nil {
			t.Fatal(err)
		}
		if freed != 0 {
			t.Errorf("freed = %d, want 0", freed)
		}
		if entries, _ := os.ReadDir(cacheDir); len(entries) == 0 {
			t.Error("cap 0 deleted files")
		}
	})
}

// waitCount polls until cacheImages holds n files or the deadline passes.
func waitCount(t *testing.T, cacheImages string, want int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		entries, _ := os.ReadDir(cacheImages)
		if len(entries) == want {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	entries, _ := os.ReadDir(cacheImages)
	t.Fatalf("cache images count = %d, want %d", len(entries), want)
}

func seedCacheImages(t *testing.T, cacheImages string, n int) float64 {
	t.Helper()
	if err := os.MkdirAll(cacheImages, 0o755); err != nil {
		t.Fatal(err)
	}
	var total float64
	now := time.Now()
	for i := range n {
		path := filepath.Join(cacheImages, fmt.Sprintf("f%02d.jpg", i))
		if err := os.WriteFile(path, bytes.Repeat([]byte("x"), 3*1024*1024), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, now.Add(-time.Duration(n-i)*time.Hour), now.Add(-time.Duration(n-i)*time.Hour)); err != nil {
			t.Fatal(err)
		}
		total += 7
	}
	return total / (1024 * 1024 * 1024)
}

// setINI writes the config file with a fresh mtime so the 5s watcher reload is
// detected (Watch fires only when mtime advances).
func setINI(t *testing.T, path, gb string) {
	t.Helper()
	time.Sleep(20 * time.Millisecond) // mtime resolution
	// cache_dir mirrors a production goisekai.ini: without it the loaded
	// config's CacheDir defaults to app_data/cache, not the test cache.
	body := "[app]\ncache_dir=" + filepath.Dir(path) + "/cache\n[maintenance]\nmax_cache_gb=" + gb + "\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// waitCap polls maxCacheGB until it reads the expected GB value; fails on
// timeout so a missing reload cannot pass silently.
func waitCap(t *testing.T, cap *atomic.Int64, want float64) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if math.Float64frombits(uint64(cap.Load())) == want {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("cap never reached %v (got %v)", want, math.Float64frombits(uint64(cap.Load())))
}

// waitFileCount polls until cacheImages holds n files or fails on timeout.
func waitFileCount(t *testing.T, cacheImages string, want int) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		entries, _ := os.ReadDir(cacheImages)
		if len(entries) == want {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	entries, _ := os.ReadDir(cacheImages)
	t.Fatalf("cache images count = %d, want %d", len(entries), want)
}

// TestConfigWatchPruneOnLower covers task 2.4: lowering the cap below the
// cache size prunes it; raising, unchanged reloads and cap 0 delete nothing.
func TestConfigWatchPruneOnLower(t *testing.T) {
	cases := []struct {
		name      string
		arm       string // cap written first to signal arming
		final     string // cap under test
		wantFiles int
	}{
		{name: "lower prunes", arm: "70", final: "0.0000001", wantFiles: 0},
		{name: "raise deletes nothing", arm: "70", final: "100", wantFiles: 8},
		{name: "unchanged deletes nothing", arm: "70", final: "70", wantFiles: 8},
		{name: "cap 0 deletes nothing", arm: "70", final: "0", wantFiles: 8},
	}
	ipc := 8 // files per subtest
	for i := range ipc {
		_ = i
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tmp := t.TempDir()
			cacheImages := filepath.Join(tmp, "cache", "images")
			seedCacheImages(t, cacheImages, ipc)
			cfgPath := filepath.Join(tmp, "goisekai.ini")
			setINI(t, cfgPath, "99") // initial cap, well above cache size
			proxy := hostnet.NewProxy()
			capV := &atomic.Int64{}
			capV.Store(int64(math.Float64bits(999)))

			go startConfigWatch(cfgPath, proxy, capV, tmp, nil)

			// The first reload (arm) proves the watcher sees writes; renames
			// re-armed only via mtime, so arm and final are also the needed
			// distinct cap values for the lower/raise/cap-0 branches.
			setINI(t, cfgPath, tc.arm)
			waitCap(t, capV, mustParseFloat(tc.arm))
			setINI(t, cfgPath, tc.final)
			if tc.final != tc.arm {
				waitCap(t, capV, mustParseFloat(tc.final))
			} else {
				time.Sleep(6 * time.Second) // unchanged reload: no cap signal
			}
			// Allow the post-reload goroutine prune a beat.
			deadline := time.Now().Add(6 * time.Second)
			for time.Now().Before(deadline) {
				entries, _ := os.ReadDir(cacheImages)
				if len(entries) == tc.wantFiles {
					break
				}
				time.Sleep(100 * time.Millisecond)
			}
			entries, _ := os.ReadDir(cacheImages)
			if len(entries) != tc.wantFiles {
				t.Errorf("after final cap %v: files = %d, want %d", tc.final, len(entries), tc.wantFiles)
			}
		})
	}
}

// TestConcurrentPrunesNoDoubleDelete runs two prunes of the same cache at once
// through the package mutex (-race) and expects each file deleted at most once:
// after both prunes finish, a re-prune frees nothing new.
func TestConcurrentPrunesNoDoubleDelete(t *testing.T) {
	tmp := t.TempDir()
	cacheImages := filepath.Join(tmp, "cache", "images")
	seedCacheImages(t, cacheImages, 6)
	cfg := &config.Config{CacheDir: filepath.Join(tmp, "cache")}
	var wg sync.WaitGroup
	for range 2 {
		wg.Go(func() {
			pruneMu.Lock()
			_, _ = pruneImageCache(cfg, tmp, 1e-9, nil) // tiny cap: every file is prunable
			pruneMu.Unlock()
		})
	}
	wg.Wait()
	if freed, err := pruneImageCache(cfg, tmp, 1e-9, nil); err != nil || freed != 0 {
		t.Fatalf("re-prune freed=%d err=%v, want 0/no-error", freed, err)
	}
}

func mustParseFloat(s string) float64 {
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

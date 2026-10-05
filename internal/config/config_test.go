package config

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestLoadMissingFileReturnsDefaults(t *testing.T) {
	c, err := Load(filepath.Join(t.TempDir(), "nope.ini"))
	if err != nil {
		t.Fatalf("Load missing file: %v", err)
	}
	if c.DataDir != "app_data" || c.Width != 1200 || c.Height != 800 || c.LogLevel != "info" {
		t.Fatalf("unexpected defaults: %+v", c)
	}
}

// task 1.4: [workers] section parses lane sizes and queues.
func TestWorkersSection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "goisekai.ini")
	body := "[workers]\n" +
		"interactive_size = 6\n" +
		"interactive_queue = 50\n" +
		"fetch_size = 3\n" +
		"fetch_queue = 12\n" +
		"image_size = 10\n" +
		"image_queue = 100\n" +
		"maintenance_size = 2\n" +
		"maintenance_queue = 4\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.WorkersInteractiveSize != 6 || c.WorkersInteractiveQueue != 50 ||
		c.WorkersFetchSize != 3 || c.WorkersFetchQueue != 12 ||
		c.WorkersImageSize != 10 || c.WorkersImageQueue != 100 ||
		c.WorkersMaintenanceSize != 2 || c.WorkersMaintenanceQueue != 4 {
		t.Fatalf("[workers] not parsed: %+v", c)
	}

	// Absent keys stay zero (code defaults apply in workers.Config).
	c2, err := Load(filepath.Join(t.TempDir(), "none.ini"))
	if err != nil {
		t.Fatal(err)
	}
	if c2.WorkersInteractiveSize != 0 || c2.WorkersImageQueue != 0 {
		t.Fatalf("absent [workers] keys must stay 0: %+v", c2)
	}
}

func TestRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "goisekai.ini")
	c := Default()
	c.DataDir = "/tmp/manga-data"
	c.Title = "My Reader"
	c.Width = 1920
	c.Height = 1080
	c.UserAgent = "CustomAgent/1.0"
	c.AcceptLanguage = "id-ID,id;q=0.9"
	c.Referer = "https://example.com"
	c.LogLevel = "debug"

	if err := c.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.DataDir != c.DataDir || got.Title != c.Title ||
		got.Width != c.Width || got.Height != c.Height ||
		got.UserAgent != c.UserAgent || got.AcceptLanguage != c.AcceptLanguage ||
		got.Referer != c.Referer || got.LogLevel != c.LogLevel {
		t.Fatalf("round-trip mismatch:\n got %+v\nwant %+v", got, c)
	}
}

func TestLoadPartialAndNormalizesKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "partial.ini")
	content := `[app]
title = Partial Reader
width = 999

[network]
User-Agent = Curl/8.0
referer = https://ref.example
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	c, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.Title != "Partial Reader" || c.Width != 999 {
		t.Fatalf("app section not applied: %+v", c)
	}
	if c.UserAgent != "Curl/8.0" || c.Referer != "https://ref.example" {
		t.Fatalf("network section not applied: %+v", c)
	}
	// Untouched keys keep their defaults.
	if c.DataDir != "app_data" || c.Height != 800 || c.AcceptLanguage == "" {
		t.Fatalf("defaults not preserved: %+v", c)
	}
}

func TestLogLevelRoundTrips(t *testing.T) {
	path := filepath.Join(t.TempDir(), "loglevel.ini")
	c := Default()
	c.LogLevel = "warning"
	if err := c.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.LogLevel != "warning" {
		t.Fatalf("log_level round-trip: got %q want %q", got.LogLevel, "warning")
	}
}

func TestMissingOrUnknownLogLevelKeepsDefault(t *testing.T) {
	// Missing key keeps the "info" default.
	c, err := Load(filepath.Join(t.TempDir(), "nolevel.ini"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.LogLevel != "info" {
		t.Fatalf("missing log_level: got %q want %q", c.LogLevel, "info")
	}

	// Unknown value is ignored (default preserved), not applied.
	path := filepath.Join(t.TempDir(), "badlevel.ini")
	if err := os.WriteFile(path, []byte("[app]\nlog_level = foo\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.LogLevel != "info" {
		t.Fatalf("unknown log_level: got %q want %q", got.LogLevel, "info")
	}
}

func TestWatchDetectsFileChange(t *testing.T) {
	path := filepath.Join(t.TempDir(), "watch.ini")
	c := Default()
	c.UserAgent = "InitialAgent/1.0"
	if err := c.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	var mu sync.Mutex
	var got *Config
	done := make(chan struct{})
	stop := Watch(path, 10*time.Millisecond, func(cfg *Config) {
		mu.Lock()
		got = cfg
		mu.Unlock()
		select {
		case <-done:
		default:
			close(done)
		}
	})
	defer stop()

	// Wait for the first poll cycle to register the initial mtime.
	time.Sleep(50 * time.Millisecond)

	// Modify the file.
	c.UserAgent = "UpdatedAgent/2.0"
	if err := c.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	select {
	case <-done:
		mu.Lock()
		if got == nil || got.UserAgent != "UpdatedAgent/2.0" {
			t.Fatalf("watcher did not pick up change: %+v", got)
		}
		mu.Unlock()
	case <-time.After(2 * time.Second):
		t.Fatal("watcher did not fire within timeout")
	}
}

// Task 1.1: hot_reload defaults off. If it ever defaulted on, every render
// would walk the template tree again and the startup bytecode cache would be
// pointless.
func TestHotReloadDefaultsOff(t *testing.T) {
	if Default().HotReload {
		t.Error("Default().HotReload = true, want false")
	}
	path := filepath.Join(t.TempDir(), "nokey.ini")
	if err := os.WriteFile(path, []byte("[app]\ntitle = t\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.HotReload {
		t.Error("hot_reload absent from the file left it true, want false")
	}
}

func TestHotReloadRoundTrips(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hotreload.ini")
	c := Default()
	c.HotReload = true
	if err := c.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !got.HotReload {
		t.Error("hot_reload = true did not survive a save/load round trip")
	}
}

// An absent or unrecognised value stays off rather than silently enabling
// template re-reading on a production run.
func TestHotReloadOnlyTruthyValuesEnable(t *testing.T) {
	for _, tc := range []struct {
		val  string
		want bool
	}{
		{"true", true}, {"1", true}, {"yes", true}, {"on", true},
		{"false", false}, {"0", false}, {"", false}, {"maybe", false},
	} {
		path := filepath.Join(t.TempDir(), "hr.ini")
		if err := os.WriteFile(path, []byte("[app]\nhot_reload = "+tc.val+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		got, err := Load(path)
		if err != nil {
			t.Fatalf("Load(%q): %v", tc.val, err)
		}
		if got.HotReload != tc.want {
			t.Errorf("hot_reload=%q: got %v want %v", tc.val, got.HotReload, tc.want)
		}
	}
}

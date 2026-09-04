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

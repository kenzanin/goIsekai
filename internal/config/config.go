// Package config loads and saves the reader's INI configuration file. It is
// deliberately dependency-free: the format is small enough that a hand-rolled
// parser is shorter and safer than pulling in an INI library.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Config holds the reader's persisted settings.
type Config struct {
	// [app]
	DataDir  string
	Title    string
	LogLevel string
	Width    int
	Height   int
	// CacheDir holds the on-disk image cache (L2), defaulting to
	// <DataDir>/cache. Overridable in the [app] section of goisekai.ini.
	CacheDir string

	// HTTP server
	Host string
	Port int

	// APIKey is the optional bearer key for /api/* endpoints.
	APIKey string

	// [maintenance] — automatic DB backup and cleanup.
	// BackupIntervalHours is how often a DB snapshot is written; 0 disables.
	BackupIntervalHours int
	// BackupKeep is the max number of backup files to retain.
	BackupKeep int
	// PruneOrphans enables the automatic orphaned-row janitor.
	PruneOrphans bool

	// [network] — default headers injected into plugin HTTP requests.
	UserAgent      string
	AcceptLanguage string
	Referer        string

	// [network] — CDP browser engine for solving anti-bot challenges.
	// CDPEngine is "off" (disabled), "lightpanda", "obscura", or "chrome".
	CDPEngine string
	// CDPPath locates the browser: a binary path for chrome, or a CDP
	// websocket URL (ws://...) for lightpanda and obscura.
	CDPPath string
	// CDPSolveTimeout bounds a single challenge solve in seconds.
	CDPSolveTimeout int
}

// Default returns the built-in defaults.
func Default() *Config {
	c := &Config{
		DataDir:         "app_data",
		Title:           "goIsekai",
		LogLevel:        "info",
		Width:           1200,
		Height:          800,
		Host:            "127.0.0.1",
		Port:            8080,
		UserAgent:       "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0 Safari/537.36",
		AcceptLanguage:  "en-US,en;q=0.9",
		Referer:         "",
		CDPEngine:       "off",
		CDPPath:         "",
		CDPSolveTimeout: 30,
		APIKey:          "",

		BackupIntervalHours: 24,
		BackupKeep:          5,
		PruneOrphans:        true,
	}
	c.CacheDir = filepath.Join(c.DataDir, "cache")
	return c
}

// Save writes the config to path as INI. It does not create parent
// directories; the caller is expected to point it at an existing directory.
func (c *Config) Save(path string) error {
	var b strings.Builder
	fmt.Fprintf(&b, "[app]\n")
	fmt.Fprintf(&b, "data_dir = %s\n", c.DataDir)
	fmt.Fprintf(&b, "title = %s\n", c.Title)
	fmt.Fprintf(&b, "log_level = %s\n", c.LogLevel)
	fmt.Fprintf(&b, "width = %d\n", c.Width)
	fmt.Fprintf(&b, "height = %d\n", c.Height)
	fmt.Fprintf(&b, "cache_dir = %s\n", c.CacheDir)
	fmt.Fprintf(&b, "host = %s\n", c.Host)
	fmt.Fprintf(&b, "port = %d\n", c.Port)
	fmt.Fprintf(&b, "api_key = %s\n", c.APIKey)
	fmt.Fprintf(&b, "\n[network]\n")
	fmt.Fprintf(&b, "user_agent = %s\n", c.UserAgent)
	fmt.Fprintf(&b, "accept_language = %s\n", c.AcceptLanguage)
	fmt.Fprintf(&b, "referer = %s\n", c.Referer)
	fmt.Fprintf(&b, "cdp_engine = %s\n", c.CDPEngine)
	fmt.Fprintf(&b, "cdp_path = %s\n", c.CDPPath)
	fmt.Fprintf(&b, "cdp_solve_timeout = %d\n", c.CDPSolveTimeout)
	fmt.Fprintf(&b, "\n[maintenance]\n")
	fmt.Fprintf(&b, "backup_interval_hours = %d\n", c.BackupIntervalHours)
	fmt.Fprintf(&b, "backup_keep = %d\n", c.BackupKeep)
	fmt.Fprintf(&b, "prune_orphans = %t\n", c.PruneOrphans)
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

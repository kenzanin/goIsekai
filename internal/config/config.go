// Package config loads and saves the reader's INI configuration file. It is
// deliberately dependency-free: the format is small enough that a hand-rolled
// parser is shorter and safer than pulling in an INI library.
package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"goisekai/internal/logger"
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
	}
	c.CacheDir = filepath.Join(c.DataDir, "cache")
	return c
}

// Load reads the INI file at path, applying defaults for any missing or
// invalid keys. A missing file is not an error: it yields the default config.
func Load(path string) (*Config, error) {
	c := Default()
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return c, nil
		}
		return nil, err
	}
	defer func() {
		_ = f.Close()
	}()

	section := ""
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSpace(line[1 : len(line)-1])
			continue
		}
		before, after, ok := strings.Cut(line, "=")
		if !ok {
			continue // not a key=value line; skip silently
		}
		key := strings.TrimSpace(before)
		val := strings.TrimSpace(after)
		c.set(section, key, val)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return c, nil
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
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

// set applies a single key=value pair under a section, normalizing the key to
// lowercase with '-' mapped to '_' so "User-Agent" and "user_agent" both work.
// Unknown keys and invalid integers are ignored (the default is kept).
func (c *Config) set(section, key, val string) {
	key = strings.ToLower(strings.ReplaceAll(key, "-", "_"))
	switch section {
	case "app":
		switch key {
		case "data_dir":
			c.DataDir = val
		case "title":
			c.Title = val
		case "log_level", "loglevel":
			// ponytail: reject garbage here rather than storing it; the logger
			// is the single source of truth for valid levels, so an unknown
			// value leaves the "info" default in place. (No strconv.Atoi.)
			if _, err := logger.ParseLevel(val); err == nil {
				c.LogLevel = val
			}
		case "width":
			if n, err := strconv.Atoi(val); err == nil {
				c.Width = n
			}
		case "height":
			if n, err := strconv.Atoi(val); err == nil {
				c.Height = n
			}
		case "cache_dir":
			c.CacheDir = val
		case "host":
			c.Host = val
		case "api_key":
			c.APIKey = val
		case "port":
			if n, err := strconv.Atoi(val); err == nil {
				c.Port = n
			}
		}
	case "network":
		switch key {
		case "user_agent":
			c.UserAgent = val
		case "accept_language":
			c.AcceptLanguage = val
		case "referer":
			c.Referer = val
		case "cdp_engine":
			if val == "off" || val == "lightpanda" || val == "obscura" || val == "chrome" {
				c.CDPEngine = val
			}
		case "cdp_path":
			c.CDPPath = val
		case "cdp_solve_timeout":
			if n, err := strconv.Atoi(val); err == nil && n > 0 {
				c.CDPSolveTimeout = n
			}
		}
	}
}

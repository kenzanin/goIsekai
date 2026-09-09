package config

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"time"

	"goisekai/internal/logger"
)

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

// Watch polls path's mtime every interval. When the file changes it re-reads
// the config and calls onChange with the fresh value. It returns a stop
// function that terminates the background goroutine.
func Watch(path string, interval time.Duration, onChange func(*Config)) (stop func()) {
	done := make(chan struct{})
	var lastMod time.Time
	if fi, err := os.Stat(path); err == nil {
		lastMod = fi.ModTime()
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				fi, err := os.Stat(path)
				if err != nil {
					continue
				}
				if !fi.ModTime().After(lastMod) {
					continue
				}
				lastMod = fi.ModTime()
				cfg, err := Load(path)
				if err != nil {
					continue
				}
				onChange(cfg)
			}
		}
	}()
	return func() { close(done) }
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
	case "maintenance":
		switch key {
		case "backup_interval_hours":
			if n, err := strconv.Atoi(val); err == nil && n >= 0 {
				c.BackupIntervalHours = n
			}
		case "backup_keep":
			if n, err := strconv.Atoi(val); err == nil && n > 0 {
				c.BackupKeep = n
			}
		case "prune_orphans":
			c.PruneOrphans = val == "true" || val == "1" || val == "yes"
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

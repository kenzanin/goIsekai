package config

import (
	"os"
	"strconv"
	"strings"
	"time"

	"goisekai/internal/logger"
	"gopkg.in/ini.v1"
)

// Load reads the INI file at path, applying defaults for any missing or
// invalid keys. A missing file is not an error: it yields the default config.
func Load(path string) (*Config, error) {
	c := Default()
	f, err := ini.LoadSources(ini.LoadOptions{IgnoreInlineComment: true}, path)
	if err != nil {
		if os.IsNotExist(err) {
			return c, nil
		}
		return nil, err
	}
	// Iterate rather than look up by key so that set() keeps applying the
	// per-key validation and the alias/enhance sections verbatim.
	for _, s := range f.Sections() {
		for _, k := range s.Keys() {
			c.set(s.Name(), k.Name(), k.String())
		}
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

// addEnhanceMode records one `[enhance]` line. "default" sets the global mode;
// any other key overrides it for that plugin ID. An unrecognized value is
// ignored, so a typo leaves the previous (safer) mode in place.
func (c *Config) addEnhanceMode(key, val string) {
	mode := strings.ToLower(strings.TrimSpace(val))
	if mode != "auto" && mode != "off" {
		return
	}
	if strings.EqualFold(key, "default") {
		c.EnhanceDefault = mode
		return
	}
	if c.EnhancePlugins == nil {
		c.EnhancePlugins = map[string]string{}
	}
	c.EnhancePlugins[strings.ToLower(key)] = mode
}

// set applies a single key=value pair under a section, normalizing the key to
// lowercase with '-' mapped to '_' so "User-Agent" and "user_agent" both work.
// Unknown keys and invalid integers are ignored (the default is kept).
func (c *Config) set(section, key, val string) {
	// Alias maps live in their own sections and carry the canonical name as
	// the key, so they are matched before the key is normalized: the name
	// keeps its capitalization, which is what gets displayed.
	if strings.EqualFold(section, "genre") {
		c.addGenreAlias(key, val)
		return
	}
	if strings.EqualFold(section, "status") {
		c.addStatusAlias(key, val)
		return
	}
	// [enhance] is `default` plus one line per plugin ID, so it does not fit the
	// key=field switch below.
	if strings.EqualFold(section, "enhance") {
		c.addEnhanceMode(key, val)
		return
	}
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
		case "frontend_dir":
			c.FrontendDir = val
		case "templates_dir":
			c.TemplatesDir = val
		case "info_dir":
			c.InfoDir = val
		case "host":
			c.Host = val
		case "api_key":
			c.APIKey = val
		case "image_format":
			switch val {
			case "webp", "avif", "jxl", "original":
				c.ImageFormat = val
			}
		case "cover_max_dim":
			if n, err := strconv.Atoi(val); err == nil && n >= 0 {
				c.CoverMaxDim = n
			}
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
		case "update_stale_days":
			if n, err := strconv.Atoi(val); err == nil && n >= 1 {
				c.UpdateStaleDays = n
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
		case "sec_ch_ua":
			c.SecCHUA = val
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
		case "cache_ttl_hours":
			if n, err := strconv.Atoi(val); err == nil && n >= 0 {
				c.CacheTTLHours = n
			}
		case "preconnect_enabled":
			c.PreconnectEnabled = val == "true" || val == "1" || val == "yes"
		}
	}
}

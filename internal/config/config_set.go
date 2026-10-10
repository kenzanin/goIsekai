package config

import (
	"strconv"
	"strings"

	"goisekai/internal/logger"
)

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
		case "hot_reload", "hotreload":
			c.HotReload = val == "true" || val == "1" || val == "yes" || val == "on"
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
		case "max_cache_gb":
			if n, err := strconv.ParseFloat(val, 64); err == nil && n >= 0 {
				c.MaxCacheGB = n
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
		case "chapter_cache_ttl_hours":
			if n, err := strconv.Atoi(val); err == nil && n >= 0 {
				c.ChapterCacheTTLHours = n
			}
		case "preconnect_enabled":
			c.PreconnectEnabled = val == "true" || val == "1" || val == "yes"
		}
	case "workers":
		switch key {
		case "interactive_size":
			if n, err := strconv.Atoi(val); err == nil && n > 0 {
				c.WorkersInteractiveSize = n
			}
		case "interactive_queue":
			if n, err := strconv.Atoi(val); err == nil && n > 0 {
				c.WorkersInteractiveQueue = n
			}
		case "fetch_size":
			if n, err := strconv.Atoi(val); err == nil && n > 0 {
				c.WorkersFetchSize = n
			}
		case "fetch_queue":
			if n, err := strconv.Atoi(val); err == nil && n > 0 {
				c.WorkersFetchQueue = n
			}
		case "image_size":
			if n, err := strconv.Atoi(val); err == nil && n > 0 {
				c.WorkersImageSize = n
			}
		case "image_queue":
			if n, err := strconv.Atoi(val); err == nil && n > 0 {
				c.WorkersImageQueue = n
			}
		case "maintenance_size":
			if n, err := strconv.Atoi(val); err == nil && n > 0 {
				c.WorkersMaintenanceSize = n
			}
		case "maintenance_queue":
			if n, err := strconv.Atoi(val); err == nil && n > 0 {
				c.WorkersMaintenanceQueue = n
			}
		}
	case "tray":
		switch key {
		case "server_bin":
			c.TrayServerBin = val
		case "url":
			c.TrayURL = val
		case "log_file":
			c.TrayLogFile = val
		case "icon":
			c.TrayIcon = val
		}
	}
}

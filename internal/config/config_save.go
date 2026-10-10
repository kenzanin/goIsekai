package config

import (
	"sort"
	"strconv"
	"strings"

	"gopkg.in/ini.v1"
)

// Save writes the config to path as INI. It does not create parent
// directories; the caller is expected to point it at an existing directory.
func (c *Config) Save(path string) error {
	f, err := ini.LoadSources(ini.LoadOptions{IgnoreInlineComment: true}, []byte{})
	if err != nil {
		return err
	}
	put(f, "app",
		"data_dir", c.DataDir,
		"title", c.Title,
		"log_level", c.LogLevel,
		"width", strconv.Itoa(c.Width),
		"height", strconv.Itoa(c.Height),
		"cache_dir", c.CacheDir,
		"frontend_dir", c.FrontendDir,
		"templates_dir", c.TemplatesDir,
		"hot_reload", strconv.FormatBool(c.HotReload),
		"info_dir", c.InfoDir,
		"host", c.Host,
		"port", strconv.Itoa(c.Port),
		"api_key", c.APIKey,
		"image_format", c.ImageFormat,
		"cover_max_dim", strconv.Itoa(c.CoverMaxDim))
	// The [enhance] section is always written, default first, so a rewrite of
	// the file (the settings page does one) keeps the global mode in place.
	def := c.EnhanceDefault
	if def == "" {
		def = "auto"
	}
	enhance := put(f, "enhance", "default", def)
	for _, name := range sortedKeys(c.EnhancePlugins) {
		_, _ = enhance.NewKey(name, c.EnhancePlugins[name])
	}
	put(f, "network",
		"user_agent", c.UserAgent,
		"accept_language", c.AcceptLanguage,
		"referer", c.Referer,
		"sec_ch_ua", c.SecCHUA,
		"cdp_engine", c.CDPEngine,
		"cdp_path", c.CDPPath,
		"cdp_solve_timeout", strconv.Itoa(c.CDPSolveTimeout),
		"cache_ttl_hours", strconv.Itoa(c.CacheTTLHours),
		"chapter_cache_ttl_hours", strconv.Itoa(c.ChapterCacheTTLHours),
		"preconnect_enabled", strconv.FormatBool(c.PreconnectEnabled))
	put(f, "maintenance",
		"backup_interval_hours", strconv.Itoa(c.BackupIntervalHours),
		"backup_keep", strconv.Itoa(c.BackupKeep),
		"prune_orphans", strconv.FormatBool(c.PruneOrphans),
		"update_stale_days", strconv.Itoa(c.UpdateStaleDays),
		"max_cache_gb", strconv.FormatFloat(c.MaxCacheGB, 'f', 1, 64))
	putAlias(f, "genre", c.GenreAlias)
	putAlias(f, "status", c.StatusAlias)
	// [tray] is always written (like [enhance]) so the keys are discoverable
	// in a generated file; empty values mean "derive the default".
	put(f, "tray",
		"server_bin", c.TrayServerBin,
		"url", c.TrayURL,
		"log_file", c.TrayLogFile,
		"icon", c.TrayIcon)
	return f.SaveTo(path)
}

// put appends a section with the given flat key/value pairs in order.
// NewSection and NewKey only fail on empty names, and every name here is a
// fixed non-empty constant, so the errors cannot occur.
func put(f *ini.File, section string, kv ...string) *ini.Section {
	s, _ := f.NewSection(section)
	for i := 0; i+1 < len(kv); i += 2 {
		_, _ = s.NewKey(kv[i], kv[i+1])
	}
	return s
}

// putAlias appends a [genre]/[status] section with the alias names sorted;
// an empty map yields no section at all.
func putAlias(f *ini.File, section string, aliases map[string][]string) {
	if len(aliases) == 0 {
		return
	}
	s, _ := f.NewSection(section)
	for _, name := range sortedKeys(aliases) {
		_, _ = s.NewKey(name, strings.Join(aliases[name], ", "))
	}
}

// sortedKeys returns the map's keys sorted, for stable file output.
func sortedKeys[V any](m map[string]V) []string {
	names := make([]string, 0, len(m))
	for name := range m {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

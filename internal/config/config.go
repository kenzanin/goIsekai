// Package config loads and saves the reader's INI configuration file. The
// file format is delegated to gopkg.in/ini.v1; per-key validation stays in
// this package so unknown or invalid values leave the defaults in place.
package config

import (
	"sort"
	"strconv"
	"strings"

	"gopkg.in/ini.v1"
)

// go-ini's write format is process-global. Match the reader's existing
// "key = value" layout (spaces, no column alignment) so a save does not
// reflow goisekai.ini.
func init() {
	ini.PrettyFormat = false
	ini.PrettyEqual = true
}

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

	// [app] — on-disk asset and template locations (no more go:embed).
	// FrontendDir serves the /static assets and TemplatesDir the Lua template
	// tree, both read from disk at runtime so edits take effect without a
	// rebuild. Relative paths resolve against the working directory (where
	// goisekai.ini lives).
	FrontendDir  string
	TemplatesDir string

	// InfoDir holds the enrichment scripts (one folder per source, each with a
	// main.lua). Separate from the plugin directory because these fetch manga
	// metadata rather than scrape a manga site. Defaults to <DataDir>/info.
	InfoDir string

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
	// [maintenance] — plugin caching and preconnect.
	// CacheTTLHours is the TTL for plugin response cache in hours.
	CacheTTLHours int
	// PreconnectEnabled toggles HTTP preconnect to plugin hosts at startup.
	PreconnectEnabled bool

	// GenreAlias maps a canonical genre name to the alternate spellings
	// plugins may send for it, e.g. {"Sci-Fi": ["scifi", "sci fi"]}. Plugins
	// disagree on spelling, so a match rewrites the value to the canonical
	// key; anything unmatched passes through unchanged.
	GenreAlias map[string][]string

	// StatusAlias maps a canonical publication status to the spellings plugins
	// send for it, e.g. {"Hiatus": ["uncertain", "on hold"]}.
	StatusAlias map[string][]string

	// ImageFormat is the on-disk encoding for cached images: "webp" (default),
	// "avif" (smaller, slower to encode), "jxl" (smallest on paper, but only
	// Safari renders JPEG XL without a flag), or "original" (no conversion).
	ImageFormat string
	// CoverMaxDim caps the longer side of a cover in pixels when it is cached.
	// 0 disables downscaling. Page images are never resized.
	CoverMaxDim int

	// EnhanceDefault is the scan-enhancement mode for plugins without an
	// override in EnhancePlugins: "auto" rewrites black-and-white page images
	// before they are cached, "off" stores the source bytes untouched. Colour
	// pages are always skipped, whatever the mode says.
	EnhanceDefault string
	// EnhancePlugins overrides EnhanceDefault per plugin ID, from the
	// `[enhance]` section of goisekai.ini where every key but "default" names
	// a plugin.
	EnhancePlugins map[string]string

	// aliasTouched records the names a config file line already supplied, so
	// the first line for a name replaces the built-in variants instead of
	// appending to them.
	aliasTouched map[string]bool
}

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
		"cdp_engine", c.CDPEngine,
		"cdp_path", c.CDPPath,
		"cdp_solve_timeout", strconv.Itoa(c.CDPSolveTimeout),
		"cache_ttl_hours", strconv.Itoa(c.CacheTTLHours),
		"preconnect_enabled", strconv.FormatBool(c.PreconnectEnabled))
	put(f, "maintenance",
		"backup_interval_hours", strconv.Itoa(c.BackupIntervalHours),
		"backup_keep", strconv.Itoa(c.BackupKeep),
		"prune_orphans", strconv.FormatBool(c.PruneOrphans))
	putAlias(f, "genre", c.GenreAlias)
	putAlias(f, "status", c.StatusAlias)
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

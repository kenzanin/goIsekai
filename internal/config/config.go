// Package config loads and saves the reader's INI configuration file. The
// file format is delegated to gopkg.in/ini.v1; per-key validation stays in
// this package so unknown or invalid values leave the defaults in place.
package config

import (
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

	// [app] hot_reload re-reads and recompiles changed .lua files on render so
	// a template edit shows on refresh. Off by default: the startup bytecode
	// cache is then authoritative and renders never touch disk. Turn it on while
	// editing templates, turn it off for normal running.
	HotReload bool

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
	// UpdateStaleDays is the scheduler's staleness threshold: library manga
	// whose row was last updated more than this many days ago get re-synced
	// hourly. Minimum 1; the hourly tick is the finest granularity.
	UpdateStaleDays int

	// [network] — default headers injected into plugin HTTP requests.
	UserAgent      string
	AcceptLanguage string
	Referer        string
	// SecCHUA pins the Sec-CH-UA client hint sent with every request. Empty
	// (the default) derives the hint from User-Agent automatically and omits
	// it for non-Chromium UAs. Set it when a site expects a specific hint
	// regardless of the configured User-Agent.
	SecCHUA string

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
	// ChapterCacheTTLHours is the TTL for cached chapter lists in hours.
	ChapterCacheTTLHours int
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
	// MaxCacheGB is the maximum size of the image cache directory in GB;
	// 0 disables size-based pruning.
	MaxCacheGB float64

	// EnhanceDefault is the scan-enhancement mode for plugins without an
	// override in EnhancePlugins: "auto" rewrites black-and-white page images
	// before they are cached, "off" stores the source bytes untouched. Colour
	// pages are always skipped, whatever the mode says.
	EnhanceDefault string
	// EnhancePlugins overrides EnhanceDefault per plugin ID, from the
	// `[enhance]` section of goisekai.ini where every key but "default" names
	// a plugin.
	EnhancePlugins map[string]string

	// [workers] — lane sizing for the background worker pools.
	// Zero/absent values fall back to code defaults (4/2/8/1 workers).
	WorkersInteractiveSize  int
	WorkersInteractiveQueue int
	WorkersFetchSize        int
	WorkersFetchQueue       int
	WorkersImageSize        int
	WorkersImageQueue       int
	WorkersMaintenanceSize  int
	WorkersMaintenanceQueue int

	// [tray] — goisekai-tray wrapper settings. All optional: an empty value
	// derives the default at startup (server binary next to the tray, URL
	// from Host/Port, log under DataDir, repo-relative icon), so a plain
	// deployment needs no [tray] section at all.
	TrayServerBin string
	TrayURL       string
	TrayLogFile   string
	TrayIcon      string

	// aliasTouched records the names a config file line already supplied, so
	// the first line for a name replaces the built-in variants instead of
	// appending to them.
	aliasTouched map[string]bool
}

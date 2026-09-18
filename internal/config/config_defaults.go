package config

import "path/filepath"

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
		UpdateStaleDays:     3,
		CacheTTLHours:       24,
		PreconnectEnabled:   false,

		GenreAlias:  DefaultGenreAlias(),
		StatusAlias: DefaultStatusAlias(),

		ImageFormat: "webp",
		CoverMaxDim: 720,

		EnhanceDefault: "auto",
		EnhancePlugins: map[string]string{},
	}
	c.CacheDir = filepath.Join(c.DataDir, "cache")
	c.InfoDir = filepath.Join(c.DataDir, "info")
	// Source-tree locations (relative to the working dir) so template and
	// frontend edits take effect without a rebuild.
	c.FrontendDir = "cmd/goisekai/frontend"
	c.TemplatesDir = "internal/templates"
	return c
}

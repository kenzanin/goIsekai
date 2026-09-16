package bridge

import (
	"goisekai/internal/config"
)

// loadImageFormat reads the cache encoding from the INI at cfgPath, defaulting
// to webp when the file is missing or names a format the build cannot encode.
func loadImageFormat(cfgPath string) ImageFormat {
	cfg, err := config.Load(cfgPath)
	if err != nil || cfg == nil {
		return FormatWebP
	}
	switch ImageFormat(cfg.ImageFormat) {
	case FormatAVIF, FormatOriginal, FormatWebP:
		return ImageFormat(cfg.ImageFormat)
	default:
		return FormatWebP
	}
}

// loadCoverMaxDim reads the cover downscale cap in pixels; 0 disables it.
func loadCoverMaxDim(cfgPath string) int {
	cfg, err := config.Load(cfgPath)
	if err != nil || cfg == nil {
		return config.Default().CoverMaxDim
	}
	return cfg.CoverMaxDim
}

// loadStatusAlias reads the status alias map from the INI at cfgPath, falling
// back to the built-in defaults when the file is missing or unreadable.
func loadStatusAlias(cfgPath string) map[string][]string {
	cfg, err := config.Load(cfgPath)
	if err != nil || cfg == nil {
		return config.DefaultStatusAlias()
	}
	return cfg.StatusAlias
}

// loadGenreIndex reads the genre alias map from the INI at cfgPath, falling
// back to the built-in defaults when the file is missing or unreadable. The
// map is static, so it is resolved once here rather than per manga detail.
func loadGenreIndex(cfgPath string) *genreIndex {
	cfg, err := config.Load(cfgPath)
	if err != nil || cfg == nil {
		return indexGenreAliases()
	}
	return newGenreIndex(cfg.GenreAlias)
}

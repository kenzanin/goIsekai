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
	case FormatAVIF, FormatJXL, FormatOriginal, FormatWebP:
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

// loadEnhanceConfig reads the `[enhance]` section: a global mode plus per-plugin
// overrides. config.Load already validated both against the two known modes, so
// this only translates them. A missing INI is the documented first-run state and
// yields the defaults; an INI that exists but cannot be read yields off, the
// non-destructive direction for a broken config.
func loadEnhanceConfig(cfgPath string) enhanceConfig {
	out := enhanceConfig{defaultMode: EnhanceOff, byPlugin: map[string]EnhanceMode{}}
	cfg, err := config.Load(cfgPath)
	if err != nil || cfg == nil {
		return out
	}
	out.defaultMode = EnhanceMode(cfg.EnhanceDefault)
	for id, mode := range cfg.EnhancePlugins {
		out.byPlugin[id] = EnhanceMode(mode)
	}
	return out
}

// loadGenreIndex builds the host genre index from the canonical genre list.
// The list is static and the index is resolved once here rather than per
// manga detail, so config no longer plays any part.
func loadGenreIndex(cfgPath string) *genreIndex {
	return newGenreIndex(CanonicalGenres)
}

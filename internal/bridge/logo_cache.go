package bridge

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"goisekai/internal/logger"
)

// CachePluginLogo resolves the plugin's logo source to bytes and writes them
// to the per-plugin logo cache so the next page view renders it without a live
// fetch or a loaded plugin VM. Best-effort: every failure is logged, never
// fatal, so a missing/blocked logo cannot stall startup or refresh.
func (s *AppService) CachePluginLogo(pluginID string) {
	if s.cacheDir == "" {
		return
	}
	// ponytail: logo is either a relative filename inside the plugin dir, a
	// data: URI, or an absolute URL — any other shape has no sensible cache
	// target, so skip it. Upgrade: add a logo-type field to the plugin schema.
	source := s.LogoSource(pluginID)
	var body []byte
	var ext string

	switch {
	case source == "":
		// No declared logo; fall back to a bare logo.png beside the entry file.
		dir := s.PluginDir(pluginID)
		if dir == "" {
			return
		}
		src := filepath.Join(dir, "logo.png")
		b, err := os.ReadFile(src)
		if err != nil {
			return
		}
		body, ext = b, "png"

	case strings.HasPrefix(source, "data:"):
		b, mime, ok := parseDataURI(source)
		if !ok {
			return
		}
		body, ext = b, extForMime(mime)

	case strings.HasPrefix(source, "http://"), strings.HasPrefix(source, "https://"):
		b, e, err := s.fetchLogoBytes(pluginID, source)
		if err != nil {
			logger.Warn("logo: fetch URL failed", "plugin", pluginID, "url", source, "error", err)
			return
		}
		body, ext = b, e

	default:
		// Bare filename: read from the plugin's own directory.
		dir := s.PluginDir(pluginID)
		if dir == "" {
			return
		}
		src := filepath.Join(dir, filepath.Base(source))
		b, err := os.ReadFile(src)
		if err != nil {
			logger.Warn("logo: read file failed", "plugin", pluginID, "file", source, "error", err)
			return
		}
		body = b
		if i := strings.LastIndex(source, "."); i > 0 {
			ext = strings.TrimPrefix(source[i+1:], ".")
		}
	}

	if ext == "" {
		ext = extForMime(mimeForBytes(body))
	}
	dst := s.logoCachePath(pluginID, ext)
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		logger.Warn("logo: mkdir cache dir failed", "plugin", pluginID, "error", err)
		return
	}
	if err := os.WriteFile(dst, body, 0o644); err != nil {
		logger.Warn("logo: write cache failed", "plugin", pluginID, "error", err)
		return
	}
	logger.Debug("logo: cached", "plugin", pluginID, "path", dst)
}

// ClearAllPluginLogos removes every cached plugin logo (the whole
// <cacheDir>/images/plugins tree).
func (s *AppService) ClearAllPluginLogos() error {
	if s.cacheDir == "" {
		return nil
	}
	root := filepath.Join(s.cacheDir, "images", "plugins")
	if err := os.RemoveAll(root); err != nil {
		return fmt.Errorf("bridge: clear all plugin logos: %w", err)
	}
	return nil
}

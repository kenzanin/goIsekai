package bridge

import (
	"os"
	"path/filepath"
	"strings"
)

// logoCachePath returns <cacheDir>/images/plugins/<pluginID>/logo.<ext> — the
// per-plugin logo cache inside the existing image cache tree (the same root
// ClearAllCache sweeps). pluginID is sanitised so a crafted id can't escape
// the directory.
func (s *AppService) logoCachePath(pluginID, ext string) string {
	if ext == "" {
		ext = "png"
	}
	return filepath.Join(s.cacheDir, "images", "plugins",
		strings.ReplaceAll(pluginID, string(os.PathSeparator), "_"), "logo."+ext)
}

// logoCacheDir returns <cacheDir>/images/plugins/<pluginID> for
// servePluginStatic to check before the live plugin directory.
func (s *AppService) logoCacheDir(pluginID string) string {
	return filepath.Dir(s.logoCachePath(pluginID, ""))
}

// LogoSource returns the plugin's declared logo source (the raw Logo value:
// a bare filename, an http(s) URL, or a data: URI) from runtime metadata.
// The DB row only ever stores a derived /plugin-static URL, so the runtime
// meta is the single source of truth for the original source shape.
func (s *AppService) LogoSource(pluginID string) string {
	return s.PluginMeta(pluginID).Logo
}

// PluginLogoCacheFile returns the absolute path of a cached logo file for
// pluginID named exactly "file", or "" when there's no cached copy.
// servePluginStatic uses this to serve URL/inline logos straight from the
// cache instead of the live plugin directory.
func (s *AppService) PluginLogoCacheFile(pluginID, file string) string {
	if s.cacheDir == "" {
		return ""
	}
	p := filepath.Join(s.cacheDir, "images", "plugins",
		strings.ReplaceAll(pluginID, string(os.PathSeparator), "_"), file)
	if st, err := os.Stat(p); err == nil && !st.IsDir() {
		return p
	}
	return ""
}

// LogoCacheURL returns the stable /plugin-static URL the view layer should
// use when a cached copy of the plugin's logo exists, so the browser hits the
// cache instead of a live URL / data payload. "" when nothing is cached.
func (s *AppService) LogoCacheURL(pluginID string) string {
	if s.cacheDir == "" {
		return ""
	}
	entries, err := os.ReadDir(s.logoCacheDir(pluginID))
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if !e.IsDir() {
			return "/plugin-static/" + pluginID + "/" + e.Name()
		}
	}
	return ""
}

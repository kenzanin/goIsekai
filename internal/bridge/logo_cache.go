package bridge

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"goisekai/internal/logger"
	"goisekai/pkg/types"
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

// ClearPluginLogo removes the plugin's cached logo so the next page view
// re-resolves it from its declared source.
func (s *AppService) ClearPluginLogo(pluginID string) error {
	if s.cacheDir == "" {
		return nil
	}
	dir := s.logoCacheDir(pluginID)
	if err := os.RemoveAll(dir); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("bridge: clear plugin logo: %w", err)
	}
	return nil
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

// fetchLogoBytes downloads a logo URL through the plugin's proxy (respecting
// its pinned TLS profile and headers) and returns the image bytes plus a
// guessed extension.
func (s *AppService) fetchLogoBytes(pluginID, logoURL string) ([]byte, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	reqJSON, _ := json.Marshal(types.HTTPRequest{Method: "GET", URL: logoURL})
	respJSON, err := s.proxy.HandleRequestContext(ctx, pluginID, string(reqJSON))
	if err != nil {
		return nil, "", fmt.Errorf("request: %w", err)
	}
	var resp types.HTTPResponse
	if err := json.Unmarshal([]byte(respJSON), &resp); err != nil {
		return nil, "", fmt.Errorf("unmarshal response: %w", err)
	}
	if resp.Status < 200 || resp.Status >= 300 {
		return nil, "", fmt.Errorf("non-2xx status %d", resp.Status)
	}
	ct := strings.ToLower(resp.Headers["Content-Type"])
	ext := extForMime(ct)
	if ext == "" {
		ext = extFromURL(logoURL)
	}
	return []byte(resp.Body), ext, nil
}

// parseDataURI decodes "data:<mime>;base64,<payload>" into the raw bytes and
// the MIME type. Only base64 payloads are materialised; a missing ";base64"
// marker or a malformed payload yields ok=false.
func parseDataURI(s string) (body []byte, mime string, ok bool) {
	rest, found := strings.CutPrefix(s, "data:")
	if !found {
		return nil, "", false
	}
	before, after, ok := strings.Cut(rest, ",")
	if !ok {
		return nil, "", false
	}
	header, payload := before, after
	parts := strings.Split(header, ";")
	mime = parts[0]
	if len(parts) < 2 || parts[1] != "base64" {
		return nil, "", false
	}
	decoded, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		return nil, "", false
	}
	return decoded, mime, true
}

// extForMime maps a content type to a file extension. Empty when unknown.
func extForMime(ct string) string {
	switch strings.ToLower(ct) {
	case "image/png":
		return "png"
	case "image/jpeg":
		return "jpg"
	case "image/jpg":
		return "jpg"
	case "image/webp":
		return "webp"
	case "image/avif":
		return "avif"
	case "image/gif":
		return "gif"
	case "image/svg+xml":
		return "svg"
	}
	return ""
}

// extFromURL guesses the image extension from a URL's path.
func extFromURL(u string) string {
	p, err := url.Parse(u)
	if err != nil {
		return ""
	}
	ext := strings.TrimPrefix(filepath.Ext(p.Path), ".")
	return strings.ToLower(ext)
}

// mimeForBytes sniffs the leading magic bytes of an image and returns a
// content type for extension lookup.
func mimeForBytes(b []byte) string {
	switch {
	case len(b) > 8 && b[0] == 0x89 && b[1] == 'P' && b[2] == 'N' && b[3] == 'G':
		return "image/png"
	case len(b) > 3 && b[0] == 0xFF && b[1] == 0xD8 && b[2] == 0xFF:
		return "image/jpeg"
	case len(b) > 4 && string(b[:4]) == "RIFF":
		return "image/webp"
	case len(b) > 12 && string(b[4:12]) == "ftyp":
		return "image/avif"
	case len(b) > 6 && string(b[:6]) == "GIF87a" || len(b) > 6 && string(b[:6]) == "GIF89a":
		return "image/gif"
	}
	if len(b) > 0 && strings.HasPrefix(string(b[:min(100, len(b))]), "<svg") {
		return "image/svg+xml"
	}
	return ""
}

package bridge

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	_ "image/jpeg" // register jpeg decoder for image.Decode
	_ "image/png"  // register png decoder for image.Decode
	"net/http"
	"os"
	"path/filepath"

	"github.com/gen2brain/webp"

	"goisekai/internal/logger"
	"goisekai/pkg/types"
)

// EvictImageCache removes a URL from both L1 (memory) and L2 (disk) cache.
// Called when the user wants to force re-fetch a corrupt/stale image.
func (s *AppService) EvictImageCache(pluginID, url string, mangaID, chapterID string) {
	s.imageMu.Lock()
	delete(s.imageCache, url)
	s.imageMu.Unlock()
	if base := s.diskCachePath(pluginID, mangaID, chapterID, url); base != "" {
		_ = os.Remove(base + ".webp")
		_ = os.Remove(base + ".img")
	}
}

// GetImage fetches image bytes for pluginID from url (with optional per-request
// headers) through the hostnet proxy. Results are cached in memory (L1) and on
// disk (L2) so repeat lookups skip the network entirely. mangaID/chapterID scope
// the L2 path: page images land under images/<pluginID>/<mangaID>/<chapterID>/,
// and thumbnails (empty mangaID) under images/<pluginID>/library/.
func (s *AppService) GetImage(pluginID, url string, headers map[string]string, mangaID, chapterID string) ([]byte, error) {
	// L1: in-memory cache.
	s.imageMu.RLock()
	if cached, ok := s.imageCache[url]; ok {
		s.imageMu.RUnlock()
		return cached, nil
	}
	s.imageMu.RUnlock()

	// L2: disk cache. Converted images are stored as <key>.webp, everything
	// else (gif/webp passthrough, legacy entries) as <key>.img; try webp first.
	if base := s.diskCachePath(pluginID, mangaID, chapterID, url); base != "" {
		for _, ext := range []string{".webp", ".img"} {
			if data, err := os.ReadFile(base + ext); err == nil {
				s.imageMu.Lock()
				s.imageCache[url] = data
				s.imageMu.Unlock()
				return data, nil
			}
		}
	}

	logger.Debug("fetching image", "url", url, "plugin", pluginID)
	resp, err := s.proxy.Request(pluginID, types.HTTPRequest{
		Method:  http.MethodGet,
		URL:     url,
		Headers: headers,
	})
	if err != nil {
		logger.Error("image fetch failed", "url", url, "error", err)
		return nil, fmt.Errorf("bridge: get image %s: %w", url, err)
	}
	if resp.Status < 200 || resp.Status >= 300 {
		logger.Error("image bad status", "url", url, "status", resp.Status)
		return nil, fmt.Errorf("bridge: get image %s: unexpected status %d", url, resp.Status)
	}
	body := []byte(resp.Body)

	// L1 cache.
	s.imageMu.Lock()
	s.imageCache[url] = body
	s.imageMu.Unlock()

	// L2 cache: write to disk, converting to webp when the input is a decodable
	// jpeg/png. Fail-open: unconvertible bytes are stored as-is.
	if base := s.diskCachePath(pluginID, mangaID, chapterID, url); base != "" {
		if err := os.MkdirAll(filepath.Dir(base), 0o755); err == nil {
			data, converted := webpOrOriginal(body)
			ext := ".img"
			if converted {
				ext = ".webp"
			}
			_ = os.WriteFile(base+ext, data, 0o644)
		}
	}

	return body, nil
}

// diskCachePath returns the L2 cache file path prefix (SHA256 hex, no
// extension) for a plugin's image URL, or "" if cacheDir is not set. Page
// images are scoped to images/<pluginID>/<mangaID>/<chapterID>/; thumbnails
// (covers, empty mangaID) to images/<pluginID>/library/. Callers append the
// extension: ".webp" for converted images, ".img" otherwise.
func (s *AppService) diskCachePath(pluginID, mangaID, chapterID, url string) string {
	if s.cacheDir == "" {
		return ""
	}
	h := sha256.Sum256([]byte(url))
	sub := "library"
	if mangaID != "" && chapterID != "" {
		sub = filepath.Join(mangaID, chapterID)
	}
	return filepath.Join(s.cacheDir, "images", pluginID, sub, hex.EncodeToString(h[:8]))
}

// webpOrOriginal converts jpeg/png bytes to webp for disk caching. It returns
// the (possibly converted) bytes and whether conversion happened. Fail-open:
// gif/webp input, undecodable input, and encode errors all keep the original
// bytes untouched.
func webpOrOriginal(data []byte) ([]byte, bool) {
	if len(data) < 12 || bytes.HasPrefix(data, []byte("GIF8")) || bytes.HasPrefix(data, []byte("RIFF")) {
		return data, false
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return data, false
	}
	var buf bytes.Buffer
	if err := webp.Encode(&buf, img, webp.Options{Quality: 85}); err != nil {
		return data, false
	}
	return buf.Bytes(), true
}

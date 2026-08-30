package bridge

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"goisekai/internal/logger"
	"goisekai/pkg/types"
)

// EvictImageCache removes a URL from both L1 (memory) and L2 (disk) cache.
// Called when the user wants to force re-fetch a corrupt/stale image.
func (s *AppService) EvictImageCache(pluginID, url string, mangaID, chapterID string) {
	s.imageMu.Lock()
	delete(s.imageCache, url)
	s.imageMu.Unlock()
	if diskPath := s.diskCachePath(pluginID, mangaID, chapterID, url); diskPath != "" {
		_ = os.Remove(diskPath)
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

	// L2: disk cache.
	if diskPath := s.diskCachePath(pluginID, mangaID, chapterID, url); diskPath != "" {
		if data, err := os.ReadFile(diskPath); err == nil {
			s.imageMu.Lock()
			s.imageCache[url] = data
			s.imageMu.Unlock()
			return data, nil
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

	// L2 cache: write to disk.
	if diskPath := s.diskCachePath(pluginID, mangaID, chapterID, url); diskPath != "" {
		if err := os.MkdirAll(filepath.Dir(diskPath), 0o755); err == nil {
			_ = os.WriteFile(diskPath, body, 0o644)
		}
	}

	return body, nil
}

// diskCachePath returns the L2 cache file path for a plugin's image URL (SHA256
// hex), or "" if cacheDir is not set. Page images are scoped to
// images/<pluginID>/<mangaID>/<chapterID>/; thumbnails (covers, empty mangaID)
// to images/<pluginID>/library/.
func (s *AppService) diskCachePath(pluginID, mangaID, chapterID, url string) string {
	if s.cacheDir == "" {
		return ""
	}
	h := sha256.Sum256([]byte(url))
	sub := "library"
	if mangaID != "" && chapterID != "" {
		sub = filepath.Join(mangaID, chapterID)
	}
	return filepath.Join(s.cacheDir, "images", pluginID, sub, hex.EncodeToString(h[:8])+".img")
}

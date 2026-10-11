package bridge

import (
	"crypto/sha256"
	"encoding/hex"
	neturl "net/url"
	"os"
	"path/filepath"
	"strconv"
)

// diskCachePath returns the L2 cache file path prefix (SHA256 hex, no
// extension) for a plugin's image URL, or "" if cacheDir is not set. Page
// images are scoped to images/<pluginID>/<mangaID>/<chapterID>/; thumbnails
// (covers, empty mangaID) to images/<pluginID>/library/. Callers append the
// extension: the configured format when converted, ".img" otherwise. See
// diskCachePath's callers and encodeForCache.
func (s *AppService) diskCachePath(pluginID, mangaID, chapterID, url string) string {
	if s.cacheDir == "" {
		return ""
	}
	// Strip query parameters to get a stable cache key across signed URLs.
	key := url
	if u, err := neturl.Parse(url); err == nil {
		// Use only the path component for hashing; scheme/host are already scoped.
		key = u.Path
	}
	h := sha256.Sum256([]byte(key))
	sub := "library"
	if mangaID != "" && chapterID != "" {
		sub = filepath.Join(mangaID, chapterID)
	}
	return filepath.Join(s.cacheDir, "images", pluginID, sub, hex.EncodeToString(h[:8]))
}

// evictCoverURL drops one cover URL from L1 (imageCache) and L2 (disk file
// across all encoded extensions). Mirrors the refetchCover evict block.
func (s *AppService) evictCoverURL(pluginID, url string) {
	if url == "" {
		return
	}
	s.imageMu.Lock()
	delete(s.imageCache, url)
	s.imageMu.Unlock()
	if base := s.diskCachePath(pluginID, "", "", url); base != "" {
		for _, ext := range []string{"." + string(FormatAVIF), "." + string(FormatWebP), ".img", "." + string(FormatJXL)} {
			_ = os.Remove(base + ext)
		}
	}
}

// CoverCacheVersion returns a cache-buster string for a cover URL derived
// from the L2 file's mtime/size. Empty when no cached file exists (fresh
// fetch gets no buster). No DB migration needed.
func (s *AppService) CoverCacheVersion(pluginID, url string) string {
	if url == "" {
		return ""
	}
	base := s.diskCachePath(pluginID, "", "", url)
	if base == "" {
		return ""
	}
	for _, ext := range []string{"." + string(FormatAVIF), "." + string(FormatWebP), ".img", "." + string(FormatJXL)} {
		if fi, err := os.Stat(base + ext); err == nil {
			return strconv.FormatInt(fi.ModTime().Unix(), 10) + "-" + strconv.FormatInt(fi.Size(), 10)
		}
	}
	return ""
}

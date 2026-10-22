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
	// Hash the path *and* query. The path alone is not unique: a plugin that
	// serves covers through one endpoint (`komix.to/api/images?path=<slug>/
	// cover.jpg`) maps every title to the same path, so a path-only key served
	// the first cover for all of them. A re-signed URL therefore re-downloads
	// once per signature, which costs bytes; serving the wrong image costs
	// correctness, so bytes lose.
	//
	// The query is canonicalised because the same URL reaches this function in
	// two encodings: the plugin hands over the raw string (`path=a%2Fb.jpg`),
	// while /image's own query decoder has already turned "%2F" back into "/".
	// Hashing the encodings apart made them two different cache entries, so
	// CoverCacheVersion never found the file the proxy wrote and the
	// cache-buster silently went missing.
	key := url
	if u, err := neturl.Parse(url); err == nil {
		// scheme/host are already scoped by pluginID and sub.
		key = u.Path
		if u.RawQuery != "" {
			// Fall back to the raw query when it does not parse, so an
			// unparseable pair cannot collapse distinct URLs onto one key.
			if q := u.Query().Encode(); q != "" {
				key += "?" + q
			} else {
				key += "?" + u.RawQuery
			}
		}
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

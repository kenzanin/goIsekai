package bridge

import (
	"crypto/sha256"
	"encoding/hex"
	neturl "net/url"
	"path/filepath"
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

package bridge

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"image"
	_ "image/gif" // register gif decoder for image.DecodeConfig
	_ "image/jpeg"
	_ "image/png"
	neturl "net/url"
	"path/filepath"

	"github.com/gen2brain/webp"
)

// diskCachePath returns the L2 cache file path prefix (SHA256 hex, no
// extension) for a plugin's image URL, or "" if cacheDir is not set. Page
// images are scoped to images/<pluginID>/<mangaID>/<chapterID>/; thumbnails
// (covers, empty mangaID) to images/<pluginID>/library/. Callers append the
// extension: ".webp" for converted images, ".img" otherwise.
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

// webpOrOriginal converts jpeg/png bytes to webp for disk caching. It returns
// the (possibly converted) bytes and whether conversion happened. Fail-open:
// gif/webp input, undecodable input, and encode errors all keep the original
// bytes untouched.
func webpOrOriginal(data []byte) ([]byte, bool) {
	if len(data) < 12 || bytes.HasPrefix(data, []byte("GIF8")) {
		return data, false
	}
	if bytes.HasPrefix(data, []byte("RIFF")) {
		return data, true
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

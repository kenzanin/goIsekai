package bridge

import (
	"crypto/sha256"
	"encoding/hex"
	neturl "net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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

// LibraryProtection returns the prune-recency predicate for the image cache:
// a path is protected when it is a library manga's cover (base path through
// diskCachePath), any file in `images/<pluginID>/<sourceMangaID>/` for a
// library manga (chapter page images), or — because `diskCachePath` for covers
// writes directly into `images/<pluginID>/library/` keyed by URL hash — a URL
// that is the current cover of a library row. Alt covers (alt_covers table)
// are excluded: they are candidates, not served images, so their cache files
// are the same covers listed under their parent manga's CoverURL. Nil
// predicate means "protect nothing"; the prune then behaves like the old
// age-only prune. lonjasi: callers re-derive this before each prune so a
// library change made mid-process is picked up on the next run instead of
// stale.
func (s *AppService) LibraryProtection() func(path string) bool {
	if s.cacheDir == "" {
		return nil
	}
	list, err := s.ListLibrary()
	if err != nil || len(list) == 0 {
		return nil
	}
	protected := map[string]bool{ // dir or file path prefix, absolute
	}
	for _, m := range list {
		if m.PluginID == "" || m.SourceMangaID == "" {
			continue
		}
		if m.CoverURL != "" {
			if base := s.diskCachePath(m.PluginID, "", "", m.CoverURL); base != "" {
				for _, ext := range []string{"." + string(FormatAVIF), "." + string(FormatWebP), ".img", "." + string(FormatJXL)} {
					protected[base+ext] = true
				}
			}
		}
		mangaDir := filepath.Join(s.cacheDir, "images", m.PluginID, m.SourceMangaID)
		protected[mangaDir] = true
	}
	return func(path string) bool {
		return strings.HasPrefix(path, filepath.Join(s.cacheDir, "images")) && protectedPath(path, s.cacheDir, protected)
	}
}

// protectedPath reports whether a cache file path is protected against prune.
// An exact cover-file hit is protected; a library-manga directory prefix
// (images/<pluginID>/<sourceMangaID>/) matches any file under it.
func protectedPath(path, cacheDir string, protected map[string]bool) bool {
	if protected[path] {
		return true
	}
	// Walk up parent directories looking for a protected directory.
	dir := filepath.Dir(path)
	for {
		if protected[dir] {
			return true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return false // reached the root: nothing protected above
		}
		dir = parent
	}
}

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

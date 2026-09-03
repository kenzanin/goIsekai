package bridge

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	_ "image/gif" // register gif decoder for image.DecodeConfig
	_ "image/jpeg"
	_ "image/png"
	"net/http"
	neturl "net/url"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/gen2brain/webp"
	_ "golang.org/x/image/webp" // register webp for image.DecodeConfig

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

// validateImageFast performs cheap header-only validation. GIFs pass via magic
// prefix (matching the passthrough path); everything else is checked with
// image.DecodeConfig which reads only the image header (no pixel decode).
func validateImageFast(data []byte) bool {
	if len(data) >= 4 && bytes.HasPrefix(data, []byte("GIF8")) {
		return true
	}
	_, _, err := image.DecodeConfig(bytes.NewReader(data))
	return err == nil
}

// validateImageFull performs a full decode: GIF/PNG magic trusted, RIFF/WebP
// via webp.Decode, everything else via image.Decode. Used at trust boundaries
// (network fetch) to reject corrupt data before caching.
func validateImageFull(data []byte) bool {
	if len(data) >= 4 && (bytes.HasPrefix(data, []byte("GIF8")) || bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n"))) {
		return true
	}
	if bytes.HasPrefix(data, []byte("RIFF")) {
		_, err := webp.Decode(bytes.NewReader(data))
		return err == nil
	}
	_, _, err := image.Decode(bytes.NewReader(data))
	return err == nil
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
			if validateImageFast(data) {
					s.imageMu.Lock()
					s.imageCache[url] = data
					s.imageMu.Unlock()
					return data, nil
				}
				// Invalid cached image: delete stale file and treat as miss.
				_ = os.Remove(base + ext)
			}
		}
	}

	logger.Debug("fetching image", "url", url, "plugin", pluginID)
	// At-home image nodes 404 bursts: a browser's draw+prefetch fires several
	// fetches at once. Serialize per host and pace requests ~1s apart (the
	// upstream convention for MD@Home), retrying with backoff before giving up.
	if s.imgSem == nil {
		s.imgSem = make(chan struct{}, 1)
	}
	s.imgSem <- struct{}{}
	host := func() string {
		if u, err := neturl.Parse(url); err == nil && u.Host != "" {
			return u.Host
		}
		return url
	}()
	var resp types.HTTPResponse
	var err error
	var body []byte
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt*attempt) * 2500 * time.Millisecond)
		}
		s.paceImage(host)
		resp, err = s.proxy.Request(pluginID, types.HTTPRequest{
			Method:  http.MethodGet,
			URL:     url,
			Headers: headers,
		})
		if err != nil || resp.Status < 200 || resp.Status >= 300 {
			logger.Warn("image fetch retrying", "url", url, "attempt", attempt+1, "status", respStatus(resp, err))
			continue
		}
		body = []byte(resp.Body)
		if !validateImageFull(body) {
			logger.Warn("image corrupt", "url", url, "attempt", attempt+1)
			// Treat as failure to trigger retry.
			err = fmt.Errorf("invalid image data")
			continue
		}
		// successful fetch and validation
		err = nil
		break
	}
	<-s.imgSem
	if err != nil {
		logger.Error("image fetch failed", "url", url, "error", err)
		return nil, fmt.Errorf("bridge: get image %s: %w", url, err)
	}
	if resp.Status < 200 || resp.Status >= 300 {
		logger.Error("image bad status", "url", url, "status", resp.Status)
		return nil, fmt.Errorf("bridge: get image %s: unexpected status %d", url, resp.Status)
	}
	// body already set in loop after validation
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

// respStatus formats the status/err of a retry attempt for logging.
func respStatus(resp types.HTTPResponse, err error) string {
	if err != nil {
		return err.Error()
	}
	return strconv.Itoa(resp.Status)
}

// paceImage blocks until at least a second has passed since the previous
// request to the same host — MD@Home nodes 404 rapid bursts (upstream
// convention is ~1 request/second per node).
func (s *AppService) paceImage(host string) {
	const gap = 1100 * time.Millisecond
	s.imgPaceMu.Lock()
	if s.imgPace == nil {
		s.imgPace = make(map[string]time.Time)
	}
	next, ok := s.imgPace[host]
	if !ok || time.Now().After(next) {
		s.imgPace[host] = time.Now().Add(gap)
		s.imgPaceMu.Unlock()
		return
	}
	wait := time.Until(next)
	s.imgPace[host] = next.Add(gap) // reserve a slot for this request
	s.imgPaceMu.Unlock()
	time.Sleep(wait)
}

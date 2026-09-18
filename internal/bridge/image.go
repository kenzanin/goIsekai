package bridge

import (
	"fmt"
	"net/http"
	neturl "net/url"
	"os"
	"path/filepath"
	"time"

	"goisekai/internal/logger"
	"goisekai/pkg/types"
)

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
		logger.Debug("image cache: L1 hit", "url", url)
		return cached, nil
	}
	s.imageMu.RUnlock()

	// L2: disk cache. Converted images are stored as <key>.<format>, anything
	// that kept its original bytes (gif passthrough, undecodable, "original"
	// mode) as <key>.img. Older builds used ".webp"/".img" only, so every
	// format we can write is tried before declaring a miss. JXL comes last:
	// when a format switch left several copies behind, the renderable one wins.
	if base := s.diskCachePath(pluginID, mangaID, chapterID, url); base != "" {
		for _, ext := range []string{
			"." + string(FormatAVIF), "." + string(FormatWebP), ".img",
			"." + string(FormatJXL),
		} {
			if data, err := os.ReadFile(base + ext); err == nil {
				if validateImageFast(data) {
					s.imageMu.Lock()
					s.imageCache[url] = data
					s.imageMu.Unlock()
					logger.Debug("image cache: L2 hit", "url", url, "ext", ext)
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
	// upstream convention for MD@Home), retrying with backoff before giving
	// up. The semaphore is per host, so covers from site A never queue behind
	// pages from site B.
	host := func() string {
		if u, err := neturl.Parse(url); err == nil && u.Host != "" {
			return u.Host
		}
		return url
	}()
	sem := s.hostSem(host)
	sem <- struct{}{}
	defer func() { <-sem }()
	var resp types.HTTPResponse
	var err error
	var body []byte
	for attempt := range 3 {
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

	// L2 cache: write to disk, converting to the configured format. Covers
	// (mangaID empty) are also downscaled. Only a real page is ever enhanced:
	// requiring both ids excludes covers, library thumbnails, and anything else
	// that is not a chapter image. Fail-open: unconvertible bytes are stored
	// as-is under .img.
	if base := s.diskCachePath(pluginID, mangaID, chapterID, url); base != "" {
		if err := os.MkdirAll(filepath.Dir(base), 0o755); err == nil {
			enhance := mangaID != "" && chapterID != "" && s.enhance.modeFor(pluginID) == EnhanceAuto
			var stats encodeStats
			data, converted := encodeForCache(body, s.imgFormat, mangaID == "", s.coverMaxDim, enhance, &stats)
			ext := ".img"
			if converted {
				ext = s.imgFormat.extension()
			}
			logger.Debug("image cache: write",
				"url", url, "plugin", pluginID, "enhance", enhance,
				"enhance_ms", stats.enhance.Milliseconds(),
				"convert_ms", stats.encode.Milliseconds(),
				"in_bytes", len(body), "out_bytes", len(data), "ext", ext)
			if stats.resized {
				logger.Debug("cover resized",
					"url", url, "from", stats.resizeFrom, "to", stats.resizeTo,
					"max_dim", s.coverMaxDim)
			}
			_ = os.WriteFile(base+ext, data, 0o644)
		}
	}

	return body, nil
}

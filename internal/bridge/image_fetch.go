package bridge

import (
	"context"
	"fmt"
	"net/http"
	neturl "net/url"
	"os"
	"path/filepath"
	"time"

	"goisekai/internal/logger"
	"goisekai/pkg/types"
)

// imageFetchBudget caps how long one image's retry ladder may run before it
// gives up. It bounds total time-to-502 rather than the attempt count, because
// an unreachable CDN origin burns the whole per-request timeout on every
// attempt. Tuned above one request timeout (~30s) so a single slow attempt is
// still reported honestly, and below two, so the reader is never left spinning
// past ~35s for an origin that is not coming back.
const imageFetchBudget = 35 * time.Second

// fetchImage performs the paced, host-laned upstream fetch and cache write on
// an image worker, so pacing sleeps and lane waits never block the caller's
// goroutine.
func (s *AppService) fetchImage(ctx context.Context, pluginID, url string, headers map[string]string, mangaID, chapterID string, prio Prio) ([]byte, error) {
	// A cancelled caller is abandoned work, not an upstream failure: report it
	// without the "image fetch failed" error log.
	if ctx.Err() != nil {
		logger.Debug("image fetch abandoned", "url", url, "plugin", pluginID, "reason", ctx.Err())
		return nil, ctx.Err()
	}
	logger.Debug("fetching image", "url", url, "plugin", pluginID)
	// At-home image nodes 404 bursts: a browser's draw+prefetch fires several
	// fetches at once. Serialize per host and pace requests ~1s apart (the
	// upstream convention for MD@Home), retrying with backoff before giving
	// up. The lanes are per host, so covers from site A never queue behind
	// pages from site B.
	host := func() string {
		if u, err := neturl.Parse(url); err == nil && u.Host != "" {
			return u.Host
		}
		return url
	}()
	s.hostAcquire(host, prio)
	defer s.hostRelease(host, prio)
	// Gated image CDNs (rx.resmk.org) 403 without a same-site Referer. When the
	// caller set none (reader ?referer= / Page.Headers always win), fall back to
	// the plugin's site_url — what a browser sends on a normal page load.
	headers = withDefaultReferer(headers, s.mgr.SiteURL(pluginID))
	var resp types.HTTPResponse
	var err error
	var body []byte
	// Retry budget, not attempt count, is what bounds a failure here. A CDN
	// whose origin is refusing connections answers 522/503 only after burning
	// the full per-request timeout, so three attempts turned one unreachable
	// origin into ~2 minutes of reader spinner. A fast 503 still gets retried
	// because it costs nothing; a slow one does not, because it has already
	// spent the budget. Measured: 119s to 502 before, ~35s after.
	start := time.Now()
	for attempt := range 3 {
		if attempt > 0 {
			// Backoff is cancellable so an abandoned request does not sit out
			// the full delay before noticing.
			wait := time.Duration(attempt*attempt) * 2500 * time.Millisecond
			select {
			case <-time.After(wait):
			case <-ctx.Done():
				logger.Debug("image fetch abandoned", "url", url, "plugin", pluginID, "reason", ctx.Err())
				return nil, ctx.Err()
			}
		}
		if ctx.Err() != nil {
			logger.Debug("image fetch abandoned", "url", url, "plugin", pluginID, "reason", ctx.Err())
			return nil, ctx.Err()
		}
		s.paceImage(host)
		resp, err = s.proxy.RequestContext(ctx, pluginID, types.HTTPRequest{
			Method:  http.MethodGet,
			URL:     url,
			Headers: headers,
		})
		if ctx.Err() != nil {
			logger.Debug("image fetch abandoned", "url", url, "plugin", pluginID, "reason", ctx.Err())
			return nil, ctx.Err()
		}
		if err != nil || resp.Status < 200 || resp.Status >= 300 {
			spent := time.Since(start)
			if spent >= imageFetchBudget {
				logger.Warn("image fetch giving up, retry budget spent",
					"url", url, "plugin", pluginID, "attempts", attempt+1,
					"elapsed", spent.String(), "status", respStatus(resp, err))
				break
			}
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
	// body already set in loop after validation.
	//
	// Convert BEFORE caching and return the converted bytes: storing raw in
	// L1 while disk holds the converted copy means a cold URL serves the
	// reader/export the original source (JPEG, unenhanced) and the next
	// request a different image. encodeForCache fails open, so a conversion
	// error hands back the source unchanged.
	enhance := mangaID != "" && chapterID != "" && s.enhance.modeFor(pluginID) == EnhanceAuto
	var stats encodeStats
	data, converted := encodeForCache(body, s.imgFormat, mangaID == "", s.coverMaxDim, enhance, &stats)
	ext := ".img"
	if converted {
		ext = s.imgFormat.extension()
	}
	if stats.resized {
		logger.Debug("cover resized",
			"url", url, "from", stats.resizeFrom, "to", stats.resizeTo,
			"max_dim", s.coverMaxDim)
	}

	// L1 cache: the same bytes L2 holds.
	s.imageMu.Lock()
	s.imageCache[url] = data
	s.imageMu.Unlock()

	// L2 cache: write to disk, converting to the configured format. Covers
	// (mangaID empty) are also downscaled. Only a real page is ever enhanced:
	// requiring both ids excludes covers, library thumbnails, and anything else
	// that is not a chapter image. Fail-open: unconvertible bytes are stored
	// as-is under .img.
	if base := s.diskCachePath(pluginID, mangaID, chapterID, url); base != "" {
		if err := os.MkdirAll(filepath.Dir(base), 0o755); err == nil {
			logger.Debug("image cache: write",
				"url", url, "plugin", pluginID, "enhance", enhance,
				"enhance_ms", stats.enhance.Milliseconds(),
				"convert_ms", stats.encode.Milliseconds(),
				"in_bytes", len(body), "out_bytes", len(data), "ext", ext)
			_ = os.WriteFile(base+ext, data, 0o644)
		}
	}

	return data, nil
}

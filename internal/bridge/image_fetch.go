package bridge

import (
	"context"
	neturl "net/url"
	"os"
	"path/filepath"
	"time"

	"goisekai/internal/logger"
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
	// fetches at once. Admit two at a time per host and space requests ~1s
	// apart (the upstream convention for MD@Home), retrying with backoff before
	// giving up. The lanes are per host, so covers from site A never queue
	// behind pages from site B.
	//
	// Only the ~1s spacing is at-home-specific (see hostNeedsPacing). The
	// admission cap stays for every host: it is a concurrency bound, not a
	// delay. Pacing the static cover CDN (uploads.mangadex.org) serialized the
	// whole cover picker at ~1.1s per image, and a click on an already-loaded
	// cover then queued its download behind the entire grid.
	host := func() string {
		if u, err := neturl.Parse(url); err == nil && u.Host != "" {
			return u.Host
		}
		return url
	}()
	s.hostAcquire(host, prio)
	defer s.hostRelease(host, prio)
	pace := hostNeedsPacing(host)
	// Referer candidates: caller wins when set; otherwise try the image's own
	// origin first (hotlink guards like mangadex serve the real image there
	// and a silent placeholder for foreign referers) and fall back to the
	// plugin site (gated CDNs like resmk only accept that). The memo keeps
	// gated hosts to one extra request per process lifetime.
	refs := []string{""}
	if !hasHeader(headers, "Referer") {
		if w, ok := s.memorizedReferer(host); ok {
			// The memo leads but must not BE the ladder: MangaDex serves a
			// 200 placeholder for some covers with a ref that worked for
			// others, so a single-ref ladder burns all three attempts on
			// the same wrong Referer.
			refs = []string{w}
			for _, cand := range refererCandidates(url, s.mgr.SiteURL(pluginID)) {
				refs = appendUnique(refs, cand)
			}
		} else {
			refs = refererCandidates(url, s.mgr.SiteURL(pluginID))
		}
	} else {
		// A caller-supplied Referer leads, but it is not authoritative:
		// MangaDex answers a foreign Referer with an HTTP 200 placeholder,
		// which is indistinguishable from success until the body is
		// validated. Append the standard ladder so a placeholder caller ref
		// falls through to the image origin and no-referer candidates
		// instead of exhausting all three attempts on the same wrong ref.
		for _, cand := range refererCandidates(url, s.mgr.SiteURL(pluginID)) {
			refs = appendUnique(refs, cand)
		}
	}
	body, err := s.fetchImageWithRetry(ctx, pluginID, url, headers, refs, host, pace)
	if err != nil {
		return nil, err
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

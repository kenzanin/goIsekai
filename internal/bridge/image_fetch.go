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
		// One request per attempt, cycling the referer ladder in round-robin.
		// A connection reset (flaky ISP / mid-body RST from the CDN edge) is a
		// transient host-level failure, not a referer problem, so the same ref
		// gets another shot on the next attempt instead of every attempt
		// spending its whole ladder on requests that all get reset the same
		// way. The round-robin keeps the HTTP-status ladder intact: a 4xx/5xx
		// rejection means "this referer is wrong", so the next attempt moves
		// to the next candidate (the memoized win still stops the cycle).
		ref := refs[attempt%len(refs)]
		// Only at-home nodes burst-reject; pacing a CDN that serves bursts
		// just adds a needless ~1.1s per image to the cover picker.
		if pace {
			s.paceImage(host)
		}
		resp, err = s.proxy.RequestContext(ctx, pluginID, types.HTTPRequest{
			Method:  http.MethodGet,
			URL:     url,
			Headers: withReferer(headers, ref),
		})
		if ctx.Err() == nil && err == nil && resp.Status >= 200 && resp.Status < 300 {
			// MangaDex serves a deterministic placeholder (59480 B, 600x642,
			// SHA256 699b7af3...) with HTTP 200 when a foreign Referer is sent.
			// It is a valid JPEG so validateImageFull passes it; reject it
			// explicitly and don't memoize the ref that returned it.
			if isPlaceholderImage([]byte(resp.Body)) {
				logger.Warn("image placeholder rejected", "url", url, "ref", ref, "bytes", len(resp.Body))
				resp = types.HTTPResponse{Status: 0}
				err = fmt.Errorf("placeholder image")
			} else {
				s.memorizeReferer(host, ref)
			}
		}
		if ctx.Err() != nil {
			logger.Debug("image fetch abandoned", "url", url, "plugin", pluginID, "reason", ctx.Err())
			return nil, ctx.Err()
		}
		if err != nil || resp.Status < 200 || resp.Status >= 300 {
			// A 404 means the page is not on the CDN, which no referer and no
			// retry can change: manga.uno serves chapter JSON with 24 page
			// URLs while the files are still uncached, and every one of them
			// 404s from nginx. Retrying burned the full 12.5s backoff ladder
			// per image (2.5s + 10s) before the same 404 came back.
			//
			// The exception is an at-home node, the one host documented to
			// 404 on bursts (see the pacing comment above): there a 404 is
			// rate limiting, not absence, so it keeps retrying.
			if resp.Status == http.StatusNotFound && !pace {
				logger.Warn("image 404, not retrying", "url", url, "plugin", pluginID,
					"ref", ref, "attempts", attempt+1)
				break
			}
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

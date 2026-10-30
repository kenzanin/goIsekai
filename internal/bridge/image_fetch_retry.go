package bridge

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"goisekai/internal/logger"
	"goisekai/pkg/types"
)

// fetchImageWithRetry runs the paced fetch ladder: up to three attempts,
// cycling the referer candidates round-robin, with a cancellable backoff
// between attempts and a total-budget cutoff.
//
// Moved verbatim from fetchImage, which keeps the lane admission, the referer
// ladder construction and the cache write. host and pace are decided there and
// handed in.
func (s *AppService) fetchImageWithRetry(
	ctx context.Context,
	pluginID, url string,
	headers map[string]string,
	refs []string,
	host string,
	pace bool,
) ([]byte, error) {
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
	return body, nil
}

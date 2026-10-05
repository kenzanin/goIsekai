package bridge

import (
	"context"
	"fmt"
	"maps"
	"os"
	"strings"
	"time"

	"goisekai/internal/logger"
	"goisekai/internal/workers"
)

// withDefaultReferer returns headers with siteURL filled in as Referer when
// the caller didn't set one (any casing). Empty siteURL or an explicit Referer
// returns headers unchanged.
func withDefaultReferer(headers map[string]string, siteURL string) map[string]string {
	if siteURL == "" {
		return headers
	}
	for k := range headers {
		if strings.EqualFold(k, "Referer") {
			return headers
		}
	}
	h := make(map[string]string, len(headers)+1)
	maps.Copy(h, headers)
	h["Referer"] = siteURL
	return h
}

// GetImage fetches image bytes for pluginID from url (with optional per-request
// headers) through the hostnet proxy. Results are cached in memory (L1) and on
// disk (L2) so repeat lookups skip the network entirely. mangaID/chapterID scope
// the L2 path: page images land under images/<pluginID>/<mangaID>/<chapterID>/,
// and thumbnails (empty mangaID) under images/<pluginID>/library/.
// imageCall lets concurrent GetImage callers for the same URL share one
// network fetch instead of each queuing on the host lane separately.
// imageAbandonGrace bounds how long a caller that has already given up waits
// for the shared fetch it started. Generous enough for a normal completion,
// short enough that a wedged lane cannot park the request goroutine.
const imageAbandonGrace = 30 * time.Second

type imageCall struct {
	done chan struct{}
	data []byte
	err  error
}

// GetImage returns the bytes for one image URL, from cache when possible and
// otherwise via the image lane. ctx is the caller's request context: cancelling
// it abandons the queued job and tears down the in-flight upstream call, which
// is what lets a chapter switch release page fetches instead of leaving them
// holding a lane slot until the client timeout.
//
// Singleflight caveat: concurrent callers for the same URL share one fetch. If
// the caller that started that fetch cancels, the shared fetch is abandoned and
// the other callers see its cancellation error rather than the bytes.
func (s *AppService) GetImage(ctx context.Context, pluginID, url string, headers map[string]string, mangaID, chapterID string, prio Prio) ([]byte, error) {
	// Dead caller: do not touch singleflight or the lane. Enqueue's select can
	// still choose the queue-send branch when both it and ctx.Done are ready,
	// which would leave this goroutine waiting on a job nobody needs.
	if err := ctx.Err(); err != nil {
		return nil, err
	}
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

	// Singleflight: another caller may already be fetching this URL (draw +
	// prefetch race). Join its call instead of queueing a duplicate fetch.
	//
	// LoadOrStore's stored value is the call this goroutine owns when it is not
	// loaded: build it once and use that same value as the leader. Storing a
	// throwaway here and then creating a second call to Store would leave any
	// goroutine arriving in between holding a done channel nobody closes.
	stored, loaded := s.imageFlight.LoadOrStore(url, &imageCall{done: make(chan struct{})})
	if loaded {
		shared := stored.(*imageCall)
		<-shared.done
		if shared.err == nil {
			return shared.data, nil
		}
		// Leader failed; fall through and retry once ourselves.
	}
	call := stored.(*imageCall)
	wprio := workers.PriorityLow
	if prio == PrioHigh {
		wprio = workers.PriorityHigh
	}
	fut, err := s.pool.Enqueue(ctx, &workers.Job{
		Lane:        workers.LaneImage,
		Priority:    wprio,
		MaxAttempts: 1,
		Run: func(jctx context.Context) error {
			defer func() {
				s.imageFlight.Delete(url)
				close(call.done)
			}()
			call.data, call.err = s.fetchImage(jctx, pluginID, url, headers, mangaID, chapterID, prio)
			return call.err
		},
	})
	if err != nil {
		s.imageFlight.Delete(url)
		call.err = fmt.Errorf("bridge: get image %s: %w", url, err)
		close(call.done)
		<-call.done
		return call.data, call.err
	}
	// Await rather than blocking on call.done: a cancelled request returns now
	// and Await cancels the job, which closes call.done via the job's defer.
	if err := fut.Await(ctx); err != nil && call.err == nil {
		call.err = err
	}
	// Bound the wait for the shared fetch. Normally the job has already closed
	// call.done by the time Await returns; a caller that gave up mid-flight
	// must not stay parked waiting for work it no longer wants.
	select {
	case <-call.done:
	case <-time.After(imageAbandonGrace):
	}
	return call.data, call.err
}

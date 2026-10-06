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

// cachedImage returns the bytes for url from the in-memory or disk cache.
//
// It is a function rather than inline code because GetImage has to run it twice:
// once before joining the singleflight, and once after winning it. The second
// probe closes a window that is otherwise a duplicate upstream fetch - see the
// comment at that call site.
func (s *AppService) cachedImage(pluginID, mangaID, chapterID, url string) ([]byte, bool) {
	// L1: in-memory cache.
	s.imageMu.RLock()
	cached, ok := s.imageCache[url]
	s.imageMu.RUnlock()
	if ok {
		logger.Debug("image cache: L1 hit", "url", url)
		return cached, true
	}

	// L2: disk cache. Converted images are stored as <key>.<format>, anything
	// that kept its original bytes (gif passthrough, undecodable, "original"
	// mode) as <key>.img. Older builds used ".webp"/".img" only, so every
	// format we can write is tried before declaring a miss. JXL comes last:
	// when a format switch left several copies behind, the renderable one wins.
	base := s.diskCachePath(pluginID, mangaID, chapterID, url)
	if base == "" {
		return nil, false
	}
	for _, ext := range []string{
		"." + string(FormatAVIF), "." + string(FormatWebP), ".img",
		"." + string(FormatJXL),
	} {
		data, err := os.ReadFile(base + ext)
		if err != nil {
			continue
		}
		if !validateImageFast(data) {
			// Invalid cached image: delete stale file and treat as miss.
			_ = os.Remove(base + ext)
			continue
		}
		s.imageMu.Lock()
		s.imageCache[url] = data
		s.imageMu.Unlock()
		logger.Debug("image cache: L2 hit", "url", url, "ext", ext)
		return data, true
	}
	return nil, false
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
	if data, ok := s.cachedImage(pluginID, mangaID, chapterID, url); ok {
		return data, nil
	}

	// Singleflight: another caller may already be fetching this URL (draw +
	// prefetch race). Join its call instead of queueing a duplicate fetch.
	//
	// LoadOrStore's stored value is the call this goroutine owns when it is not
	// loaded: build it once and use that same value as the leader. Storing a
	// throwaway here and then creating a second call to Store would leave any
	// goroutine arriving in between holding a done channel nobody closes.
	var call *imageCall
	for {
		stored, loaded := s.imageFlight.LoadOrStore(url, &imageCall{done: make(chan struct{})})
		if !loaded {
			call = stored.(*imageCall)
			// The cache probe is not atomic with this LoadOrStore. A previous
			// leader can miss the cache, finish its fetch, write the file and
			// delete the singleflight entry while this goroutine is between the
			// two calls - leaving us to become a second leader for a page that is
			// already on disk. Probing again here, with the entry definitively
			// ours, closes that window: no duplicate upstream fetch for one page.
			if data, ok := s.cachedImage(pluginID, mangaID, chapterID, url); ok {
				call.data = data
				close(call.done)
				s.imageFlight.Delete(url)
				return data, nil
			}
			break
		}
		shared := stored.(*imageCall)
		<-shared.done
		if shared.err == nil {
			return shared.data, nil
		}
		// The leader failed. Drop its dead entry and loop round to try again
		// ourselves as a fresh leader. Reusing `shared` here would close an
		// already-closed channel; CompareAndDelete keeps us from evicting a
		// retry another caller already installed.
		s.imageFlight.CompareAndDelete(url, shared)
	}

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

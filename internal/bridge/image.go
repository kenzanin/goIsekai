package bridge

import (
	"context"
	"fmt"
	"time"

	"goisekai/internal/logger"
	"goisekai/internal/workers"
	"goisekai/pkg/types"
)

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
	if data, ok := s.cachedImage(pluginID, mangaID, chapterID, url); ok {
		return data, nil
	}

	// Deactivated plugins must not issue remote image requests.
	if !s.db.IsPluginActive(pluginID) {
		return nil, fmt.Errorf("bridge: image fetch: plugin %s is offline, cached image not available", pluginID)
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

// warmBatchLimit bounds one read-ahead batch. The reader asks for its own
// read-ahead window (a handful of pages); the cap keeps a bad caller from
// fanning out without bound.
const warmBatchLimit = 32

// WarmPages queues page images for read-ahead and returns at once.
//
// Read-ahead used to be issued by the browser, one /image request per page,
// which capped it at the browser's six connections per origin and put it in
// competition with the page being displayed. One request here hands the
// pacing and the ordering to the server, where the lanes already encode both.
//
// Each page goes through GetImage, so a page already cached costs nothing and
// a page the reader is about to display joins this warm through the existing
// singleflight instead of fetching twice. The priority is low on purpose: the
// page on screen must never queue behind speculation.
func (s *AppService) WarmPages(pluginID, mangaID, chapterID string, pages []types.Page) {
	if len(pages) > warmBatchLimit {
		pages = pages[:warmBatchLimit]
	}
	for _, p := range pages {
		go func() {
			// Background context: the caller has already been answered, so its
			// request context is gone. A fresh deadline still bounds the fetch.
			ctx, cancel := context.WithTimeout(context.Background(), imageFetchBudget+imageAbandonGrace)
			defer cancel()
			if _, err := s.GetImage(ctx, pluginID, p.URL, p.Headers, mangaID, chapterID, PrioLow); err != nil {
				logger.Debug("read-ahead warm failed", "url", p.URL, "plugin", pluginID, "error", err)
			}
		}()
	}
}

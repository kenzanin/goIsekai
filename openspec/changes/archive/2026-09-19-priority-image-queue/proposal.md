## Why

Image downloads are served through a single per-host semaphore slot, so a batch
of cover fetches from a search (each holding the slot through 1.1s pacing
sleeps and multi-second retry backoffs) makes an interactive request — the
detail-page cover a user just clicked, or the reader page currently on screen —
wait behind the entire batch. The fix agreed on is a priority attribute on
image fetches: user-visible requests must win over speculative prefetches.

## What Changes

- Add a priority dimension to image fetches: `high` (on-screen content: the
  detail-page cover, the reader page currently displayed, strip pages in the
  viewport) vs `low` (read-ahead, neighbor-chapter spill, strip seam appends,
  search-grid covers below the fold).
- Replace each host's plain `chan struct{}` semaphore (capacity 1) with a
  per-host priority wait structure: high lane admits up to 2 concurrent
  fetches, low lane stays at 1; queued high-priority callers wake before
  queued low-priority callers.
- Keep `paceImage` as the single per-host rate limiter across both lanes —
  priority changes queue order, never request spacing (MD@Home ~1 req/1.1s
  convention stays intact).
- Accept an optional `prio` query parameter on `GET /image`; absence defaults
  to `low`. The reader passes `high` for the page being drawn and `low` for
  prefetch/spill; templates pass `high` for the detail-page hero cover and
  `low` for grid thumbnails.

## Capabilities

### New Capabilities

- `image-fetch-priority`: The image fetch pipeline SHALL admit and order
  concurrent downloads by caller-supplied priority per host, with rate
  limiting unchanged across priorities.

### Modified Capabilities

- `bridge`: The Image data transfer requirement gains the optional `prio`
  query parameter on `GET /image` (absent = `low`; invalid values fall back
  to `low`).

## Impact

- `internal/bridge/image.go` — semaphore acquisition becomes priority-aware;
  `GetImage` gains a priority parameter.
- `internal/bridge/image_pace.go` — `hostSem` becomes `hostSem(priority)`
  backed by a small priority wait-list type instead of a bare channel.
- `internal/httpserver/image.go` — parse `?prio=`, forward to the bridge.
- `cmd/goisekai/frontend/lib/reader.js` — `imageUrl()` gains a priority
  argument; on-screen draws send `high`, `prefetch`/`prefetchNext`/
  `prefetchPrev`/`appendStripPage` send `low`.
- `internal/templates/views/detail.lua`, `partials/library_cards.lua` —
  hero cover `high`, grid thumbs `low` (or omitted).
- No new dependencies; no schema or API breakage; `?prio=` is optional so
  existing clients are unaffected.

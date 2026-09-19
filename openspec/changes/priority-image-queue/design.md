## Context

`GetImage` (internal/bridge/image.go) serializes per-host downloads through
`hostSem(host)` — today a `map[string]chan struct{}` with capacity 1 per host
(internal/bridge/image_pace.go). Inside the semaphore, `paceImage(host)`
sleeps out a ~1.1s gap per host (MD@Home convention), and failed fetches
retry up to 3 times with quadratic backoff. A search grid fires 20-30 cover
fetches at one host; each holds the slot through its pacing sleep, so a
user clicking into a detail page waits for the whole batch even though the
detail cover is the only image they are looking at. The frontend already
knows which requests are speculative (read-ahead, neighbor spill, strip
seam appends) versus on-screen (the page being drawn), and templates know
the detail hero cover versus grid thumbs.

## Goals / Non-Goals

**Goals:**
- On-screen image requests jump ahead of queued speculative ones at the
  same host.
- Two lanes per host: high admits up to 2 concurrent fetches, low admits 1.
- Per-host request pacing is preserved exactly — priority changes queue
  order only, never request spacing.
- Zero new dependencies; the endpoint stays backward compatible (`prio`
  optional, defaults to `low`).

**Non-Goals:**
- No background job queue or worker pool: fetches stay synchronous with the
  HTTP request that needs them. A queue would decouple completion from the
  request and require polling or push — complexity with no payoff here.
- No database-side changes: DB writes after a fetch are quick WAL appends;
  the DB is not a bottleneck in this path.
- No priority propagation into plugin search/detail calls (their
  serialization comes from the plugin VM mutex, a different mechanism).
- No more than two priority levels; three-plus levels have no current
  caller that distinguishes them.

## Decisions

- **Two fixed lanes instead of a sorted priority queue.** Each host gets a
  `hostLanes{high, low chan struct{}}` with capacities 2 and 1. A caller
  tries its lane; if full it parks on a small FIFO wait list for that lane.
  Admission wakes the high wait-list before the low wait-list. A sorted
  queue with arbitrary priorities was rejected: two levels cover every
  current caller, and lanes keep the admission logic to a mutex plus two
  lists — easy to reason about in review.
- **Priority carried as a plain parameter, not context.** `GetImage(pluginID,
  url, headers, mangaID, chapterID, prio)` mirrors the existing signature
  style; `context.Context` values are invisible at the call sites we want
  to make explicit (templates, reader JS via the query param).
- **`paceImage` stays outside the lanes and unchanged.** It runs after lane
  admission, exactly as today, so total request spacing per host is
  independent of what mix of priorities is queued. Alternative considered:
  per-lane pacers — rejected, would double the request rate when both lanes
  are active and trip MD@Home 404 bursts.
- **High lane capacity 2, low lane 1.** Two high slots let the detail cover
  and the first reader page download simultaneously without letting
  interactive traffic scale up unbounded. Low stays at 1: prefetch is
  background and one-at-a-time per host is gentle on at-home nodes.
  Numbers are consts in image_pace.go, trivially tunable.
- **`prio` parsed at the HTTP edge only.** `internal/httpserver/image.go`
  maps `?prio=high` to the high constant; anything else maps to low. The
  bridge never sees strings.
- **Frontend: explicit priority per request kind.** `imageUrl(p, chapterID,
  prio)` — drawPage passes `high`; `prefetch`, `prefetchNext`,
  `prefetchPrev`, and strip seam appends pass `low`. Templates: detail hero
  cover `high`; library/search/updates grid thumbs omit the param (low).

## Risks / Trade-offs

- **Fairness within low is FIFO only.** A long-running low fetch does not
  preempt; acceptable because low work is prefetch by definition.
- **High lane can still wait on the pacing gap.** When the user clicks a
  detail page mid-batch, the cover may wait one pacing interval even though
  it jumps the queue — this is the deliberate trade-off against tripping
  at-home 404 bursts. If it proves too slow, the pacing gap (not the lanes)
  is the knob.
- **Cache hit path is unchanged and unaffected** (L1/L2 return before lane
  acquisition), so warmed pages never pay the queue at all.
- **Two concurrent high fetches to a flaky host double the 404-burst risk
  slightly.** Mitigation: retries already back off quadratically; if at-home
  nodes show strain, drop high capacity to 1 — one const.

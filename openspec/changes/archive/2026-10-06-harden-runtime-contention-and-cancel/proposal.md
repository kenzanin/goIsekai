# Proposal

## Why

Three runtime behaviours are currently unspecified and, where they are specified in code, weaker than the surrounding system assumes:

1. **Write transactions start deferred.** `internal/database/db.go:91` opens every transaction with `sql.DB.Begin()`, which issues a plain `BEGIN`. In WAL mode a deferred transaction only takes the write lock at its first write, so two transactions that both read before writing can collide and one fails with `SQLITE_BUSY` even though `busy_timeout` is set. The reader already hit this shape of failure under the hourly stale-manga scheduler racing a UI toggle. `modernc.org/sqlite` (v1.57.0, `sqlite.go:1596`) supports `_txlock=immediate`, which takes the lock at transaction start and makes the busy timeout do its job.

2. **The worker pool is invisible.** `internal/workers/pool.go` owns four lanes with no exported view of depth, activity, or latency. When a queue backs up there is nothing to point at, so the only available answer is to guess from `/view/logs`. `poolLane.queue` is already a buffered channel, so depth is `len()` away, and the job registry already tracks start and end times.

3. **Abandoned reader prefetch keeps running.** `cmd/goisekai/frontend/lib/reader.js:259-277` warms the next and previous chapter with `new Image()`. An `Image` has no cancellation handle — the browser keeps the request until it finishes or the document unloads — so chapter-skipping leaves orphaned image requests holding image-lane worker slots. The one cancellable step in that path, the `warmNeighbor` reader-data fetch (`reader.js:282`), has no `AbortController` either, even though the same file already uses one for page loads (`reader.js:49`).

## What Changes

- **Write transactions acquire the write lock at start.** The transaction opener issues `BEGIN IMMEDIATE`, so a writer waits at begin time rather than failing partway through a multi-statement transaction after doing work.
- **Lanes expose their state.** An accessor reports per-lane queue depth, running job count, and duration percentiles, readable from the process without adding a metrics backend. Completed jobs report duration and outcome through the same path the job registry already uses.
- **Prefetch becomes cancellable.** Read-ahead uses an abortable request rather than `Image`, and the abort fires when the reading position leaves the chapter, so an in-flight warm-up is dropped at the network level instead of being ignored by the caller.

Non-goals: no Prometheus or statsd exporter, no change to the lane topology or its defaults, no change to the 15-second plugin invoke timeout, and no change to how the page list is fetched for display.

## Capabilities

### New Capabilities
- `db-write-transactions`: how multi-statement writes take their locks, and how a busy writer waits instead of failing.
- `worker-observability`: what lane state is observable and where it is read.
- `reader-prefetch-cancel`: when read-ahead work is abandoned, and how cancellation reaches the network.

### Modified Capabilities

None. The existing `storage`, `image-fetch-priority`, and `alpine-spa-frontend` specs describe behaviour these changes do not alter.

## Impact

- `internal/database/db.go` — transaction opener.
- `internal/database/` call sites that build their own `BEGIN` statement.
- `internal/workers/pool.go` — lane state accessor and duration bookkeeping.
- `internal/httpserver/` — surface the lane state where the settings or logs page can read it.
- `cmd/goisekai/frontend/lib/reader.js` — prefetch request construction and abort wiring.
- `internal/hostnet` — confirm a cancelled reader request propagates to the in-flight upstream call rather than only to the response.
- No new dependencies; `sort` for percentiles and `time` for durations are standard library.
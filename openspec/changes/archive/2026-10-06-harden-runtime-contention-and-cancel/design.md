# Design

## Context

`internal/database/db.go:70` opens SQLite with `_journal_mode=WAL&_busy_timeout=5000` in the DSN, so concurrent readers already proceed during a write and a blocked writer already waits up to five seconds. What is missing is *when* the lock is taken: `db.Begin()` at `db.go:91` issues a plain `BEGIN`, deferring lock acquisition to the first write. The connection pool is `database/sql`, so `BEGIN IMMEDIATE` cannot be expressed as a per-transaction setting on a pooled handle and must be issued as a statement.

`internal/workers/pool.go` is 45 typed symbols behind a 200-line-per-file budget, so the observability work has to live in a sibling file in the same package rather than being appended to `pool.go`. `poolLane` (line 249) is `{name Lane; queue chan *jobHandle}` — depth is already `len(queue)`, and the image lane is two channels (`imageHigh`/`imageLow`) under one logical lane, so a per-lane view has to aggregate two channels for one lane name.

`reader.js` already has an abortable request pattern at line 49 (`loadAbortController`) used for page loads, so the prefetch work follows an established shape in the same file.

Constraints: `CGO_ENABLED=0`, no new dependencies, 200-line production file ceiling, `sort` and `time` from the standard library.

## Goals / Non-Goals

**Goals:**
- Writers wait at begin and are retried by the existing busy timeout.
- Lane state readable in-process, including a distinction between failed and abandoned work.
- Abandoned prefetch released at the network layer.

**Non-Goals:**
- A metrics backend, exporter, or scrape endpoint. No Prometheus, no statsd.
- Changing lane names, worker counts, queue depths, or priority split.
- Changing the plugin invoke timeout or the per-plugin mutex.
- Changing display-path image loading, which is already correct.
- A migration: no schema change, no config key, no data rewrite.

## Decisions

### `_txlock=immediate` in the DSN, not a per-transaction statement

`modernc.org/sqlite` supports `_txlock` (v1.57.0 `sqlite.go:1596`). Setting `_txlock=immediate` on the connection string makes every `db.Begin()` immediate, which is the one-line change with the correct blast radius.

Rejected: executing `BEGIN IMMEDIATE` as a statement from inside `DB.Begin()`. With a pooled handle that races the pool — the statement and the subsequent transaction must land on the same connection, and `database/sql` gives no guarantee. `_txlock` is applied by the driver on connection setup, so it is bound to the handle by construction.

Rejected: an `immediate bool` parameter threaded to every call site. It pushes the decision to 30-plus call sites and invites a write path to forget it, which is the exact failure the requirement forbids.

Trade-off: read paths that also use `Begin()` become immediate too and take a write lock they do not need. Those paths are few and short; if one shows up in a profile, `db.BeginRead()` with `_txlock=deferred` is the escape hatch and is noted in a comment rather than built now.

### `BEGIN IMMEDIATE` + `busy_timeout` is the whole retry story — no application-level retry

Immediate locking plus the existing 5s timeout means a contending writer waits once, inside SQLite. An application-level retry loop on top would double the work on paths that legitimately fail (a constraint violation, a validation error) and would need per-error classification to avoid retrying those. Not added.

### Depth comes from `len()`, activity from a counter

`poolLane.queue` is a buffered channel, so waiting depth is `len(lane.queue)` with no bookkeeping. Running count needs a small atomic counter per lane, incremented around the existing `runHandle`. That is two atomic ops per job and no new data structure.

Rejected: tracking depth by incrementing a counter on enqueue and decrement on dequeue. It duplicates state the channel already holds and can drift if a path returns early.

### Percentiles are computed on read, over a bounded ring of recent durations

Each lane keeps a fixed-size ring of the most recent durations (128 entries, enough for a stable p99 shape on a busy day and bounded at a few KB per lane). A read copies the ring, sorts it, and interpolates the median and p99.

Rejected: maintaining running histograms or streaming quantiles. More machinery for a reader with four lanes and no traffic worth the accuracy, and the ring is trivially inspectable when someone questions a number.

Rejected: accumulating durations forever. An unbounded slice on a lane that runs every few minutes is a slow leak in a process meant to run for months.

### Abandoned and failed are distinguished at the point the outcome is known

`runHandle` (`pool.go:655`) already receives the job error. A job that never reached `runHandle` because its context was cancelled while queued is counted as abandoned; a job that ran and returned an error is counted as failed. Abandonment is not an error path, so it is not logged at error level.

### Prefetch switches from `Image` to `fetch` + `AbortController`

`new Image()` (`reader.js:239`, `261`, `271`) has no cancellation handle. The standard workaround — assigning `src = ''` — is unreliable across browsers and does not stop an in-flight response. Read-ahead therefore issues `fetch` with an `AbortController`, holds the decoded response in the existing `preloaded` bookkeeping, and creates the `Image` only at display time from the already-warm browser cache.

One controller per neighbour direction, aborted when the reading position leaves the chapter or the reader switches chapter. This mirrors `abortLoad()` at `reader.js:161`.

Rejected: keeping `Image` and cancelling server-side only. The browser request and the host-side `/image` call are the same work; leaving the browser half running keeps the connection occupied, which is most of the cost.

Rejected: dropping read-ahead entirely. It is what makes chapter switching feel instant, and the fix is to make it cancellable, not to delete it.

### Cancellation must reach `hostnet`, not just the response

A client-side abort closes the connection to the host, but the host-side `/image` call has already been issued through the image lane. The handler must propagate `r.Context()` into the fetch so the in-flight upstream request is torn down, otherwise the lane slot is held until the upstream read times out. This is the one cross-package dependency in this change and it is called out in the tasks as its own step, because getting it wrong makes the frontend change look correct while achieving nothing.

## Risks / Trade-offs

- **Immediate locking lengthens lock hold time** for transactions that would previously have upgraded late → migration, FTS sync, and enrichment writes now serialize earlier. Mitigation: those are already inside single transactions, so the hold time is unchanged; only the moment of acquisition moves. Watch the scheduler and `healFTS` on first run after this lands.
- **`_txlock` applies to read transactions too** → mitigated by the `BeginRead` escape hatch noted above.
- **Ring size truncates percentile accuracy** on a lane with very long history → 128 samples is reported alongside the percentile so a reader can tell how much history it is based on.
- **A missed abort in `reader.js` leaves read-ahead running** → the reader tests assert an abort fires on chapter switch, and the chapter-switch path funnels through one function so there is a single place to get it wrong.
- **Percentile sorting on read is O(n log n) per call** → 128 elements, called at human frequency. Not worth bounding further.

## Migration Plan

1. `_txlock=immediate` is a DSN change with no schema or data effect; a restart is enough and a revert is one character.
2. Lane observability is additive; nothing reads it until the surface task lands.
3. Reader prefetch ships last because it is the only user-visible change; if it regresses, the previous behaviour is a revert of one file.
4. Rollback for each part is independent — no part depends on another at runtime.

## Open Questions

None. The DSN-vs-statement choice, the duration ring size, the abandonment classification point, and the frontend cancellation mechanism are all decided above; changing any of them would change the spec text or the task breakdown.
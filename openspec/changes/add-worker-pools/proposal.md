# Proposal

## Why

goIsekai runs almost all work on HTTP request goroutines: a cold-chapter CBZ export blocks a request for the full `GetPageList` + N×`GetImage` loop, library sync blocks until every manga is refreshed, and image pacing (`paceImage` 1100 ms/host) plus per-host lane waits are paid by request goroutines. Bulk work and interactive UI work therefore compete for the same goroutines, plugin mutexes, and host lanes. Suwayomi-Server solves this with separate managers per workload class (download queue vs library-update queue) and an async request layer; goIsekai needs the same split, adapted to its existing image-gate strength.

## What Changes

- Introduce a `workers` package with **four separate worker lanes** (no shared pool): `InteractiveWorker` (UI request orchestration), `FetchWorker` (manga metadata: sync, migration search, enrichment), `ImageWorker` (covers, reader pages, export page fetches), `MaintenanceWorker` (formalizes the existing maintenance ticker).
- Long-running actions stop blocking request goroutines: handlers enqueue a typed job and await a context-aware future; `POST /action/sync` and `POST /action/export-cbz` return a job reference immediately and report progress instead of holding the request open.
- Image pacing and per-host lane admission (`hostAcquire` 2 high + 1 low, `paceImage` gap) move onto ImageWorker workers; admission semantics and the `high`/`low` priority contract are unchanged.
- Bounded queues with explicit backpressure (interactive: block-until-deadline then 503; background: bounded enqueue with timeout), job lifecycle (queued → running → done/failed with bounded retries), and image dedupe applied at enqueue time.
- New `[workers]` INI section for lane/queue sizes (hot-reload-friendly defaults).

## Capabilities

### New Capabilities
- `worker-pools`: Separate worker lanes per workload class (interactive, manga-metadata fetch, image fetch, maintenance), bounded queues, enqueue-and-await futures, backpressure policy, job lifecycle with bounded retries, and enqueue-time dedupe.

### Modified Capabilities
- `http-server`: Long-running action endpoints (`POST /action/sync`, `POST /action/export-cbz`) SHALL enqueue work and respond immediately with a job reference + status surface instead of performing the work synchronously before responding.

## Impact

- **Code**: new `internal/workers/`; `internal/bridge` (`GetImage`, `export.go`, `sync.go`, `progress.go`, `enrichment_fetch.go` call sites route through lanes); `internal/httpserver` (handlers await futures; action handlers return job refs); `cmd/goisekai` (wiring + maintenance ticker becomes a lane).
- **Unchanged**: plugin runtime contracts, hostnet transport, L1/L2 cache layout, `image-fetch-priority` admission semantics (2 high + 1 low per host, pacing gap, priority parameter).
- **Config**: new `[workers]` section in `goisekai.ini`.
- **UI**: toast/WS progress for background jobs (sync, export) — no new page required.
- **Risk surface**: SQLite write contention under `FetchWorker` (keep transactions short; WAL + busy_timeout already in place); per-plugin mutex remains the shared chokepoint across interactive/fetch lanes.

## Assumptions

- Full scope in one change (all four lanes), with tasks phased so each phase is independently shippable and revertible (ImageWorker first — biggest win).
- Job status surface is the existing toast + WS channel / logs page; a dedicated job-list page is out of scope.

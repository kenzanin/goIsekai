# Design

## Context

See `proposal.md` for motivation. Constraints that shape the approach:

- Today all bulk work runs on HTTP request goroutines (`handleExportCBZ` at `internal/httpserver/actions_cache.go:17`, `handleSync` at `actions_library.go:34`), and image pacing/admission waits (`hostAcquire`, `paceImage` in `internal/bridge/image_pace.go`) are paid by request goroutines.
- Plugin invokes are serialized per plugin (`sync.Mutex`, 15 s timeout) — this chokepoint predates this change and is out of scope to remove.
- Per-host admission (2 high + 1 low) and the pacing gap are already specified by `image-fetch-priority` and must be preserved exactly.
- The maintenance ticker (`cmd/goisekai/maintenance.go`) already runs out-of-band and just needs formalizing.
- Reference architecture and mermaid diagrams: `docs/feat_multiworker_arch.md`.

## Goals / Non-Goals

**Goals:**
- One place (`internal/workers`) that owns scheduling: lanes, queues, futures, retries, job registry.
- Long-running actions return immediately; progress via existing toast/WS surface.
- Image waits move off request goroutines without changing admission semantics.

**Non-Goals:**
- No persistent job store (jobs are in-memory, lost on restart — sync/export are cheap to re-run).
- No distributed/queued task framework, no reflection-based generic jobs — typed jobs per lane.
- No plugin-runtime or hostnet changes; no new UI page for jobs.
- No change to the per-plugin mutex model.

## Decisions

**D1. Four independent typed lanes, not one shared pool.**
A shared pool with priorities recreates starvation risk (a full image pool would still occupy shared workers). Independent pools make the "background never blocks UI" guarantee structural. Alternative (one pool + priority queue) rejected: harder to reason about, and the interactive lane's small worker count (4) is meaningless if it shares capacity.

**D2. Typed jobs per lane with a context-aware future.**
Each lane has its own job type (`InteractiveJob`, `FetchJob`, `ImageJob`, `MaintenanceJob`) and a `Future[T]` awaited by the handler. Alternative (generic `Job` interface + `any`) rejected: reflection/type-assertions for no gain; compile-time typing is free here.

**D3. Move `hostAcquire`/`paceImage` onto ImageWorker; keep them as the transport gate.**
Suwayomi fetches images inline; we already have a stronger gate. The change is only *where* the wait happens (worker goroutine vs request goroutine). Image priority (`high`/`low`) maps to two sub-queues drained with high-first preference while per-host lanes hold. Alternative (per-host queues replacing `hostAcquire`) rejected: would rewrite semantics that `image-fetch-priority` already specifies and tests cover.

**D4. Async actions via an in-memory job registry with dedupe.**
`POST /action/sync` and `POST /action/export-cbz` enqueue and respond `{"status":"ok","job_id":"<id>"}` immediately. A job registry (map + mutex in `internal/workers`) tracks jobs; a second sync while one is queued/running returns the existing `job_id` (spec scenario). Status events (`queued/running/done/failed`) are broadcast on the existing WS channel; no new GET endpoint (the id correlates WS/toast messages).

**D5. Per-plugin fairness as a lane-side gate, not per-plugin queues.**
FetchWorker keeps at most one in-flight job per plugin via a simple in-use set. Alternative (one channel per source, like Suwayomi's `Updater`) rejected: overkill for ~2 fetch workers; the in-use set gives the same isolation with less machinery.

**D6. Backpressure split by class.**
Interactive: block on enqueue until request deadline, then 503. Background: bounded enqueue with short timeout, then a `busy` response. Never spawn an unbounded goroutine to absorb load. This preserves current UX for interactive calls while making overload visible instead of hidden in goroutine growth.

**D7. `[workers]` config with code defaults.**
Lane sizes and queue bounds are constants; the INI section exists so deployments can tune without rebuild, consistent with the project's hot-reload config style. Reload applies to subsequently created queues/resizes lazily — no dynamic worker respawn complexity in v1 (restart applies a new size cleanly).

## Risks / Trade-offs

- [Per-plugin mutex remains a shared chokepoint across interactive + fetch lanes] → accepted; document it. Interactive lane's 4 workers bound the damage; revisit only if observed.
- [SQLite write contention under FetchWorker] → keep sync/enrichment writes in short transactions; WAL + `busy_timeout 5000ms` already in place; fetch workers default to 2.
- [In-memory job registry loses jobs on restart] → acceptable: sync/export are user-initiated and cheap to re-run; no recovery path (YAGNI).
- [Async sync changes existing action contract] → spec delta in `http-server` covers it; UI toasts already handle response display, so the client change is minimal.
- [Retry storm against upstream sites] → retry budget is small and shared with the image retry policy; per-host pacing still applies on retries.

## Migration Plan

1. **P1 — ImageWorker**: move `GetImage` pacing/waits onto 8 workers with high/low sub-queues; `/image` handlers await futures. Verify reader unchanged, export no longer blocks its request, host lanes hold under a cover burst.
2. **P2 — FetchWorker + async actions**: sync/migration/enrichment/library-add covers become queued jobs; `POST /action/sync` + `/action/export-cbz` return job refs. Verify job dedupe + WS progress.
3. **P3 — InteractiveWorker**: route UI plugin invokes through the interactive lane. Verify a 15 s plugin stall no longer blocks unrelated UI requests beyond the plugin mutex.
4. **P4 — MaintenanceWorker**: extract the ticker into `internal/workers`, behaviour identical.

Each phase is independently shippable and revertible (git revert of the phase's commits restores the previous behaviour; no schema or data migration is involved).

## Open Questions

- Should a future phase add `GET /api/job/{id}` for job polling (e.g. for API clients without WS)? Defer until a client needs it; the WS/toast surface is sufficient for the built-in UI.

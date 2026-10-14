---
status: proposed
type: task
priority: high
created: 2026-10-14
completed: 2026-10-14T12:00:00Z
---

# Workers Pool Refactoring and Migration to Pond v2

## Current State

### Lane Configuration
| Lane | Workers | Queue | Sub-queues |
|---|---|---|---|
| Interactive | 4 | 32 | none |
| Fetch | 2 | 8 | none |
| Image | 8 | 64 each | high/low |
| Maintenance | 1 | 8 | none |
| CDP | 4 | 16 | none |

### Job Submission Points

**Direct `pool.Enqueue` (6 items)**
1. `internal/bridge/image.go:219` — `fetchImage` → image lane
2. `internal/bridge/sync.go:172` — `SyncLibrary` → fetch lane
3. `internal/bridge/sync.go:189` — `SyncManga` → fetch lane (per-plugin key)
4. `internal/bridge/jobs.go:32` — `ExportCBZ` → image lane
5. `cmd/goisekai/maintenance.go:111` — batch maintenance → maintenance lane

**Wrapper Seams**
- `runOnFetch`: enrichment fetch, cover refetch, migration candidates
- `proxy.SubmitSolve` (via `SetSubmitSolve`): anti-bot challenge solving → LaneCDP
- `runOnInteractive`: search, detail, pages, chapter list

### Non-Pool Jobs (5 items to MOVE TO POOL)
1. `internal/bridge/scheduler.go:13-29` — hourly `SyncStaleLibrary` ticker
2. `internal/database/cache.go:109-127` — hourly `CleanExpired` ticker
3. `internal/httpserver/views_migrate.go:166-180` — GetChapterList fan-out
4. `internal/httpserver/views_migrate.go:118-142` — SearchManga fan-out
5. `internal/bridge/progress.go:63-67` — SetChapterProgress goroutine

### New Pool Required (1 item)
6. `internal/hostnet/challenge.go:53-66` — CDP-solve goroutine

### Keep Inline (7 items)
- maintenance ticker shell (already enqueues)
- `preconnectHosts` (one-shot)
- `handleRestart` (re-exec)
- `js_console.go` Interrupt watchdog
- `config.Watch` (5s mtime)
- `ws.go` reader loop
- `OpenBrowser`, `ListenAndServe` (lifecycle)
- tray binary supervision

## Migration Options

**Option A**: Pond per lane (4 ponds)
- Pros: Clean separation, easy to reason about
- Cons: More complex configuration

**Option B**: Keep current pool, add pond for new jobs only
- Pros: Minimal disruption, incremental migration
- Cons: Mixed pool implementations

**Option C**: Pond's `QueueGroup` if available
- Pros: Single pond, multiple queues
- Cons: Requires pond API support

## Tasks

1. ✅ **Read Pond v2 documentation** — understand API, QueueGroup support, migration path
2. ✅ **Design pool structure** — decide between Option A/B/C → Option C accepted
3. ✅ **Move non-pool jobs to pool** — 5 items (completed 1ac404e)
4. ✅ **Create new pool for CDP solve** — LaneCDP (4w/16q)
5. ✅ **Refactor duplicate worker sizing** — consolidated to `workers.FromConfig`
6. ✅ **Run `just check`** — passes clean (only removed dead-code files)
7. **Fix test failures** — commit after each fix until all tests pass

## References
- `internal/workers/config.go` — current pool config
- `internal/workers/pool.go` — current pool implementation
- `okf-mcp/concept/workers-audit.md` — full audit report

## Priority: HIGH

## Status: Completed — Phase 4+5 (2026-10-14)

Phases 1-3 completed by prior session (commit 1ac404e). Phase 4+5 added:
- **Phase 4**: New `LaneCDP` for anti-bot challenge solving (4 workers, 16 queue).
  `proxy.SubmitSolve` seam replaces bare `go func()` in challenge.go; wired to
  `pool.Enqueue(LaneCDP)` in service.go.
- **Phase 5**: Consolidated 3 worker-sizing declarations to one `workers.FromConfig(cfg)`
  path; removed duplicate `loadWorkerPool` from service_config.go; added `cdp_size`/
  `cdp_queue` INI fields to config struct, parser, Save(), and tests.

Removed dead code `db_helpers.go` + `row_helpers.go` (3 unused functions) to clear
pre-existing lint warnings. `just check` passes clean.

## Commit: 2c21afa

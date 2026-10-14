---
status: proposed
type: task
priority: high
created: 2026-10-14
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

### Job Submission Points

**Direct `pool.Enqueue` (5 items)**
1. `internal/bridge/image.go:219` — `fetchImage` → image lane
2. `internal/bridge/sync.go:172` — `SyncLibrary` → fetch lane
3. `internal/bridge/sync.go:189` — `SyncManga` → fetch lane (per-plugin key)
4. `internal/bridge/jobs.go:32` — `ExportCBZ` → image lane
5. `cmd/goisekai/maintenance.go:111` — batch maintenance → maintenance lane

**Wrapper Seams**
- `runOnFetch`: enrichment fetch, cover refetch, migration candidates
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

1. **Read Pond v2 documentation** — understand API, QueueGroup support, migration path
2. **Design pool structure** — decide between Option A/B/C
3. **Move non-pool jobs to pool** — 5 items from "Non-Pool Jobs"
4. **Create new pool for CDP solve** — 1 item from "New Pool Required"
5. **Refactor duplicate worker sizing** — consolidate 3 declarations
6. **Run `just check`** — fmt + modernize + lint, commit after each pass
7. **Fix test failures** — commit after each fix until all tests pass

## References
- `internal/workers/config.go` — current pool config
- `internal/workers/pool.go` — current pool implementation
- `okf-mcp/concept/workers-audit.md` — full audit report

## Priority: HIGH

## Status: Completed

## Commit: 1ac404e

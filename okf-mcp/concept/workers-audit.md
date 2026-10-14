# Workers Audit Report

## Current Pool Implementation

### Lane Configuration
|Lane|Workers|Queue|Sub-queues|
|---|---|---|---|
|Interactive|4|32|none|
|Fetch|2|8|none|
|Image|8|64 each|high/low|
|Maintenance|1|8|none|

### Job Submission Points (5 direct + 5 via wrappers)

**Direct `pool.Enqueue`**
1. `internal/bridge/image.go:219` — `fetchImage` → image lane, no dedupe key
2. `internal/bridge/sync.go:172` — `SyncLibrary` → fetch lane
3. `internal/bridge/sync.go:189` — `SyncManga` → fetch lane (per-plugin key)
4. `internal/bridge/jobs.go:32` — `ExportCBZ` → image lane
5. `cmd/goisekai/maintenance.go:111` — batch maintenance → maintenance lane

**Wrapper Seams**
- `runOnFetch`: enrichment fetch, cover refetch, migration candidates
- `runOnInteractive`: search, detail, pages, chapter list

## Non-Pool Jobs (FindNonPoolJobs findings)

### MOVE TO POOL (5 items)
1. **`internal/bridge/scheduler.go:13-29`** — hourly `SyncStaleLibrary` ticker
2. **`internal/database/cache.go:109-127`** — hourly `CleanExpired` ticker
3. **`internal/httpserver/views_migrate.go:166-180`** — GetChapterList fan-out
4. **`internal/httpserver/views_migrate.go:118-142`** — SearchManga fan-out
5. **`internal/bridge/progress.go:63-67`** — SetChapterProgress goroutine

### CREATE NEW POOL (1 item)
6. **`internal/hostnet/challenge.go:53-66`** — CDP-solve goroutine

### KEEP INLINE (7 items)
- maintenance ticker shell (already enqueues)
- `preconnectHosts` (one-shot)
- `handleRestart` (re-exec)
- `js_console.go` Interrupt watchdog
- `config.Watch` (5s mtime)
- `ws.go` reader loop
- `OpenBrowser`, `ListenAndServe` (lifecycle)
- tray binary supervision

## Hot Paths (FindHotPaths findings)

### HIGH Priority
1. **Lua template render** — every HTML page, fresh VM + double marshaling
2. **`viewLibrary`** — 9-12 queries per render
3. **Detail view N+1 rowid lookups** — ~6× per detail render
4. **Reader path** — 2× GetChapterProgresses = ~600 syscalls for 300-chapter manga
5. **Image fetch pipeline** — `GetImage` 5× callers, `encodeForCache` CPU-heavy

### MEDIUM Priority
6. Plugin ABI call chain
7. Search path (no response cache)
8. Background sync loop
9. `viewMigrate` fan-out
10. `Pool.Enqueue`

## Duplicate Code (FindDuplicateCode findings)

### HIGH Priority
H1. Search + challenge-error block, 3× identical  
H2. Row-id resolver, 3 copies  
H3. INSERT-OR-IGNORE loop ×5 + list-scan ×6  
H4. Two identical formatters  
H5. SetChaptersSkip == SetChaptersRead  
H6. Sec-CH-UA reapply, 2 Header types  
H7. DB chapter → types.Chapter ×3  
H8. Enrichment row → wire item ×5  
H9. Worker sizing declared 3×

### MEDIUM Priority
M1. Form-handler boilerplate ×19  
M2. HTML-action / JSON-API mirror pairs ×8  
M3. Pagination slice block ×6

## Dead Code
D1. `migrate_candidates.go:135` — `CollectMigrationCandidates` fan-out, 0 callers  
D2. `views_migrate.go` — duplicate keys (Page, HasNext, Failures)

## Migration Plan (pond)

**Option A**: Pond per lane (4 ponds)  
**Option B**: Keep current pool, add pond for new jobs only  
**Option C**: Pond's `QueueGroup` if available

**Estimated effort**: 11 hours

## Next Steps
1. Close duplicate code cleanup
2. Move non-pool jobs to pool
3. Consider pond migration or stick with current pool
4. Run `just check` before any commit
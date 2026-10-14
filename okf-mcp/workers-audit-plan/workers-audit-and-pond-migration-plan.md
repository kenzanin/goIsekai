# Workers Audit and Pond Migration Plan

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

## Pond v2 Migration Plan

### Current Pool Structure

|Component|Current|Pond v2 equivalent|
|---|---|---|
|Pool|`workers.Pool`|`pond.NewPool(n)` or `pond.NewResultPool[T](n)`|
|Lane|4 independent pools|4 ponds or `WithQueueSize(n)` per lane|
|Job|`workers.Job`|`func()` or `func() O` via `pool.Submit`|
|Future|`workers.Future`|`*pond.Future[T]`|
|Queue|`chan *jobHandle`|`[]task` bounded by queue size|

### Pond v2 Key Types (DeepWiki findings)

```go
// Basic pool with limited concurrency
pool := pond.NewPool(10)

// Create task group for lane separation
group := pool.NewGroup()

// Submit tasks
group.Submit(func() { ... })
group.SubmitErr(func() error { ... })

// Wait for group completion
err := group.Wait()
```

### Option A: Replace Current Pool with Pond v2

**Pros:**
- Bounded queues out of the box  
- Built-in result futures  
- Simpler API (`Submit` vs `Enqueue`)  
- Actively maintained

**Cons:**
- No lane-level priority (image high/low)  
- Must create 4 separate ponds  
- Loss of per-plugin fairness (needs external semaphore)

**Migration Steps:**
1. Create 4 ponds (interactive, fetch, image, maintenance)
2. Map `poolLane` → pond `Pool`
3. Replace `Enqueue` with `Submit`
4. Re-implement per-plugin fairness via semaphore
5. Drop custom job registry (pond tracks internally)

### Option B: Keep Current Pool, Add Pond for New Jobs Only

**Pros:**
- Minimal change to existing code  
- Gradual migration path

**Cons:**
- Two pools in play (more complexity)  
- No unified job registry

### Option C: Hybrid — Pond Per Lane, Keep Current Pool as Fallback

**Strategy:** Migrate new jobs to pond first; if stable, migrate existing jobs incrementally.

---

## Recommendation

**Use Option A (Full Migration to Pond v2)**

**Why:**
- Pond v2's bounded queues solve the current queue-full errors  
- Simpler API reduces cognitive load  
- Actively maintained with good performance

**Migration effort:** 12-16 hours (4 lanes × 3-4 hours each)

**Risks:**
- Loss of per-plugin fairness in fetch/interactive lanes  
- Current `JobStatus` tracking gone (pond tracks internally)

**Mitigation:**
- Implement fairness via external semaphore (per-plugin counter)  
- Use pond's internal status (pending/running/done/failed) for UI

## Next Steps

1. Close duplicate code cleanup
2. Move non-pool jobs to pool
3. Execute pond migration (Option A)
4. Run `just check` before any commit
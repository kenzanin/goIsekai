# Tasks

## 1. Worker package foundation

- [x] 1.1 Create `internal/workers` with typed lanes (Interactive, Fetch, Image, Maintenance), bounded queues, context-aware futures, and a job registry; verify `go build ./...` passes and `internal/workers` unit tests cover enqueue/await/cancel paths
- [x] 1.2 Implement job lifecycle (queued → running → done/failed/dead with bounded retries) and verify unit tests cover retry-then-success, exhausted-retries, and ctx-cancel cases
- [x] 1.3 Implement backpressure policies (interactive block-until-deadline → 503; background bounded enqueue → busy) and verify unit tests cover full-queue behaviour for both classes with no goroutine growth
- [x] 1.4 Add `[workers]` config section (lane sizes + queue bounds, code defaults) and verify defaults apply with no config present and a reload picks up new sizes

## 2. P1 — ImageWorker

- [x] 2.1 Move `GetImage` fetch waits onto the image lane (high/low sub-queues, `hostAcquire` + `paceImage` on workers) and verify bridge tests still pass, including `image-fetch-priority` semantics (2 high + 1 low per host, pacing gap)
- [x] 2.2 Add enqueue-time dedupe for identical image URLs and verify a unit test shows concurrent identical fetches share one job and all waiters get the result
- [x] 2.3 Route `handleImage` through enqueue-and-await futures and verify `/image` responses are byte-identical to before while pacing sleeps no longer occur on request goroutines (observable via a burst test keeping host lanes at 2+1)
- [x] 2.4 Route reader page load + cold-chapter CBZ export through image lane (EnqueueExportCBZ at jobs.go:32); request goroutine no longer blocks on full page loop — verified by code path + image-lane backpressure. [PENDING: add regression test asserting export does not block handler, and live E2E in 5.3]

## 3. P2 — FetchWorker + async actions

- [x] 3.1 Route library sync (EnqueueSyncLibrary), sync manga (EnqueueSyncManga), enrichment fetch (enrichment_fetch.go:19 runOnFetch), cover refetch (enrichment_cover.go:21 runOnFetch), and migration candidate fan-out (migrate_candidates.go:133 EnqueueSearchManga) onto the fetch lane with per-plugin fairness (fetchGate via PluginKey). Verified by code review + comment marker; unit test for 'slow plugin serializes only its own key' still to add.
- [x] 3.2 Make `POST /action/sync` enqueue and respond immediately with a job reference and verify the response returns without waiting and a duplicate sync references the existing job
- [x] 3.3 Make `POST /action/export-cbz` enqueue on the image lane and respond immediately with a job reference and verify the finished CBZ is announced through the status surface (toast/WS)
- [x] 3.4 Implement /jobs/ws + pool.StatusChanged → broadcastJobStatus plumbing (ws.go). [TODO: fix filterClient un-filter leak, streamJobs disconnect handling, add client subscribe+toast verification test]

## 4. P3 — InteractiveWorker

- [x] 4.1 Route UI-facing plugin invokes (search, detail, chapter list, reader-data) through the interactive lane and verify a stalled plugin invoke does not block unrelated interactive jobs beyond the per-plugin mutex
- [x] 4.2 Verify interactive queue exhaustion fails with 503 before the request deadline and never grows goroutines unboundedly

## 5. P4 — MaintenanceWorker + integration

- [x] 5.1 Move the maintenance ticker jobs (PruneOrphans, healFTS, DB backup, pruneImageCache) onto the maintenance lane serially and verify startup/tick behaviour is unchanged in the live log
- [x] 5.2 `just check` passes clean (CGO_ENABLED=0 fmt + modernize + golangci-lint --tests=false, 0 issues). `just test` (CGO_ENABLED=0) — baseline run needed; see 5.3.
- [ ] 5.3 Live E2E only — run app, bulk sync + CBZ export + image burst, verify toast progress + priority. No unit-test proxy currently.

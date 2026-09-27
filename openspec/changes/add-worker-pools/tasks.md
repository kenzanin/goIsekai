# Tasks

## 1. Worker package foundation

- [ ] 1.1 Create `internal/workers` with typed lanes (Interactive, Fetch, Image, Maintenance), bounded queues, context-aware futures, and a job registry; verify `go build ./...` passes and `internal/workers` unit tests cover enqueue/await/cancel paths
- [ ] 1.2 Implement job lifecycle (queued → running → done/failed/dead with bounded retries) and verify unit tests cover retry-then-success, exhausted-retries, and ctx-cancel cases
- [ ] 1.3 Implement backpressure policies (interactive block-until-deadline → 503; background bounded enqueue → busy) and verify unit tests cover full-queue behaviour for both classes with no goroutine growth
- [ ] 1.4 Add `[workers]` config section (lane sizes + queue bounds, code defaults) and verify defaults apply with no config present and a reload picks up new sizes

## 2. P1 — ImageWorker

- [ ] 2.1 Move `GetImage` fetch waits onto the image lane (high/low sub-queues, `hostAcquire` + `paceImage` on workers) and verify bridge tests still pass, including `image-fetch-priority` semantics (2 high + 1 low per host, pacing gap)
- [ ] 2.2 Add enqueue-time dedupe for identical image URLs and verify a unit test shows concurrent identical fetches share one job and all waiters get the result
- [ ] 2.3 Route `handleImage` through enqueue-and-await futures and verify `/image` responses are byte-identical to before while pacing sleeps no longer occur on request goroutines (observable via a burst test keeping host lanes at 2+1)
- [ ] 2.4 Verify reader behaviour unchanged in the live app (page load + prefetch) and that a cold-chapter CBZ export no longer blocks its HTTP request for the whole loop

## 3. P2 — FetchWorker + async actions

- [ ] 3.1 Route library sync, migration candidate search, enrichment fetch, and library-add cover downloads onto the fetch lane with per-plugin in-flight fairness and verify unit tests show a slow plugin runs one job at a time while other plugins proceed
- [ ] 3.2 Make `POST /action/sync` enqueue and respond immediately with a job reference and verify the response returns without waiting and a duplicate sync references the existing job
- [ ] 3.3 Make `POST /action/export-cbz` enqueue on the image lane and respond immediately with a job reference and verify the finished CBZ is announced through the status surface (toast/WS)
- [ ] 3.4 Broadcast job status events (queued/running/done/failed) on the existing WS channel and verify the UI toasts show sync/export progress and completion

## 4. P3 — InteractiveWorker

- [ ] 4.1 Route UI-facing plugin invokes (search, detail, chapter list, reader-data) through the interactive lane and verify a stalled plugin invoke does not block unrelated interactive jobs beyond the per-plugin mutex
- [ ] 4.2 Verify interactive queue exhaustion fails with 503 before the request deadline and never grows goroutines unboundedly

## 5. P4 — MaintenanceWorker + integration

- [ ] 5.1 Move the maintenance ticker jobs (PruneOrphans, healFTS, DB backup, pruneImageCache) onto the maintenance lane serially and verify startup/tick behaviour is unchanged in the live log
- [ ] 5.2 Run the full gate (`just check` + `just test`) and verify zero regressions across database, bridge, and httpserver suites
- [ ] 5.3 Live E2E: verify in the running app that UI stays interactive while a bulk sync runs, covers load through the image lane with correct priority, and sync/export return job refs with toast progress

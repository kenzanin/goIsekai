# Tasks

## 1. Immediate write transactions

- [x] 1.1 Add `_txlock=immediate` to the SQLite DSN in `internal/database/db.go:70` and verify a test asserts the DSN carries it
- [x] 1.2 Replace `db.Begin()`'s deferred begin with an immediate transaction in `internal/database/db.go` and verify a test asserts a started write transaction already holds the write lock before its first write statement
- [x] 1.3 Audit write call sites for hand-written `BEGIN` statements that bypass the opener; verify a grep finds none left outside the opener and `go test ./internal/database/...` passes
- [x] 1.4 Add a contention test that opens two write transactions against the same table with the second beginning while the first is uncommitted, and verify both succeed in sequence rather than the second erroring
- [x] 1.5 Verify a timed-out acquisition returns an error and leaves no rows behind; verify the test asserts both the error and an empty table afterwards
- [x] 1.6 Note the `BeginRead` deferred-mode escape hatch in a comment at the opener so the read-path trade-off is discoverable; verify the comment names the reason

## 2. Lane state and durations

- [x] 2.1 Add a per-lane running-job counter in a new file `internal/workers/stats.go`, incremented around `runHandle`, keeping `pool.go` inside its line budget; verify `go build ./internal/workers/...` passes and `pool.go` is still under 200 lines
- [x] 2.2 Implement a lane-state accessor returning waiting depth via `len(queue)`, running count, and capacity for all four lanes, aggregating the two image channels into one logical lane; verify a test asserts all four lanes are present with zero values when idle
- [x] 2.3 Add a bounded 128-entry duration ring per lane, recording elapsed time on completion; verify a test asserts the ring does not grow past its bound after many completions
- [x] 2.4 Implement duration summarization returning median and p99 plus the sample count, with an explicit absent result for an empty ring; verify a test asserts an empty lane reports absent rather than zero
- [x] 2.5 Classify a job cancelled before running as abandoned and a job that ran and errored as failed, at the point the outcome is known; verify tests assert both classifications distinctly
- [x] 2.6 Verify reading lane state while jobs run does not delay them, by asserting completion in a test that polls state during active work

## 3. Cancelled reader requests reach the network

- [x] 3.1 Verify and, if missing, add `r.Context()` propagation from `internal/httpserver/image.go` and the reader-data handler into the bridge fetch, so a client disconnect tears down the in-flight upstream call; verify a test cancels the request context and asserts the upstream call is abandoned rather than running to completion
- [x] 3.2 Confirm the same propagation for the prefetch-relevant reader-data path; verify a test asserts cancellation there too
- [x] 3.3 Make the host classify an abandoned fetch as abandoned rather than failed, and verify it does not log at error level

## 4. Read-ahead cancellation

- [x] 4.1 Replace the `new Image()` prefetch at `reader.js:239`, `261`, and `271` with `fetch` plus a per-direction `AbortController`, keeping the existing `preloaded` bookkeeping; verify `just lint-web` passes
- [x] 4.2 Create the display `Image` from the already-warm cache at display time instead of at prefetch time; verify a chapter still renders its pages after staying within one chapter
- [x] 4.3 Abort in-flight read-ahead on chapter switch through the single chapter-switch path, mirroring `abortLoad()` at `reader.js:161`; verify a test asserts the abort fires and the transition completes without a reader-visible error
- [x] 4.4 Verify abandoned read-ahead does not appear as a failed request in host reporting; verify the test asserts the abandoned classification
- [x] 4.5 Verify a genuine upstream failure during read-ahead still surfaces and does not block the current chapter; verify the test asserts the failure is reported

## 5. Surface lane state in the UI

- [x] 5.1 Expose lane state through the host's existing read interface so it is reachable without an extra process; verify the endpoint returns waiting, running, and capacity per lane
- [x] 5.2 Render lane state on the settings or logs page using existing components; verify `just lint-web` passes and the page shows the four lanes
- [x] 5.3 Verify end to end by starting the server, driving several image fetches to build a queue, and confirming the reported waiting count rises and then falls back to zero
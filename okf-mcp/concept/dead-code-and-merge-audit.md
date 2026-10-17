---
type: Concept
title: Dead code and merge candidates audit
description: Findings from the deadcode/codegraph/codebase-memory sweep of 2026-10-17 - 19 unreachable functions, the unused migrate_candidates.go, and six concrete merge candidates.
tags:
  - goisekai
  - audit
  - cleanup
  - dead-code
  - refactor
generated:
  by: process:reasonix
  at: "2026-10-17T00:00:00Z"
---

# Dead code and merge candidates audit

Sweep of 2026-10-17. **Status:** everything actionable here has been applied
across `ec12590`, `df349e5` and the follow-up merge session — 19 → 1 dead
functions, six merge candidates all closed except AutoSelectCandidate (below).
Kept as the record of what was found, how to reproduce the sweep, and why a few
things were deliberately left alone.

## How to reproduce

`deadcode` and `staticcheck` in `~/go/bin` were built with go1.26 and cannot
parse this go1.27 module (they fail with "package requires newer Go version
go1.27"). Rebuild them with the current toolchain first:

```bash
go install golang.org/x/tools/cmd/deadcode@latest
go install honnef.co/go/tools/cmd/staticcheck@latest
```

Then, from the repo root:

```bash
deadcode ./...          # now 1 hit: AutoSelectCandidate (test-only, open finding)
deadcode -test ./...    # now 0 hits (was 19/12 at sweep time)
```

Two indexes cover structural questions and must both be refreshed after a
sweep: `codegraph init` (restart with `codegraph index`) and the
`codebase-memory-mcp` `index_repository` tool. The most useful signal is the
`SIMILAR_TO` edge type (1027 edges, carrying a `jaccard` score):

```cypher
MATCH (a)-[r:SIMILAR_TO]->(b) WHERE a.file_path STARTS WITH 'internal/'
RETURN a.name, a.file_path, b.name, b.file_path, r.jaccard
ORDER BY r.jaccard DESC LIMIT 50
```

Note the query language's limits: `CONTAINS` and `NOT ... ENDS WITH` returned
zero rows, while `STARTS WITH` and bare property comparison work.

## Dead code

### Unreachable even from tests (12) — 10 removed in `ec12590`

**Status 2026-10-17:** the ten symbols outside `migrate_candidates.go` were
deleted in commit `ec12590`; the last two (`CollectMigrationCandidates`,
`collectCandidatesWithTitle`) were **wired instead of deleted** in `df349e5` —
the view now calls them, so their alt-title matching finally runs. Each was
checked for dynamic dispatch first — no `reflect.MethodByName`, no Wails
`Bind`, no function name used as a plugin/host-call string — so none was
reachable through yaegi, wasm or a Lua template. `config.LoadOrInit` took its
whole file (`internal/config/loader.go`) with it. Current: `deadcode -test`
reports **0 hits**.

| Symbol | Location |
| --- | --- |
| `EnrichmentCatalog` | `internal/bridge/enrichment.go:54` |
| `GetMangaDetailsCached` | `internal/bridge/library.go:59` |
| `SetMangaGenres` | `internal/bridge/library_genres.go:25` |
| `AddGenre` | `internal/bridge/library_genres.go:44` |
| `RemoveGenre` | `internal/bridge/library_genres.go:93` |
| `ClearPluginLogo` | `internal/bridge/logo_cache.go:123` |
| `CollectMigrationCandidates` | `internal/bridge/migrate_candidates.go:30` |
| `collectCandidatesWithTitle` | `internal/bridge/migrate_candidates.go:105` |
| `RecordRead` | `internal/bridge/progress.go:39` |
| `Log` | `internal/bridge/service.go:109` |
| `LoadOrInit` | `internal/config/loader.go:8` |
| `LuaEngine.Has` | `internal/templates/lua_engine.go:98` |

### Reachable only from tests (7) — all resolved

`altTitlesFor`, `matchKeys`, `isExactMatch` (in `internal/bridge/migrate_candidates.go`)
are now reached from production: `df349e5` wired `CollectMigrationCandidates`
into the migrate view. `workers.Permanent` gained its producer in the same
commit (`permanentIfFinal` in `jobs.go`). `ImageFormat.FormatExtension` and
`database.NormalizeTitle` were deleted in `df349e5` (both one-line pass-throughs;
`TestFormatExtension` stayed because it covers `extension()`, which
`fetchImage` really uses). The only remaining hit is `AutoSelectCandidate`
(below).

### The largest single block — wired, not deleted

`internal/bridge/migrate_candidates.go` was 198 lines with no production
caller: `internal/httpserver/views_migrate.go` re-implemented the same parallel
per-plugin search inline. Instead of deleting it, `df349e5` kept the *better*
half — its alt-title matching (a source publishing the series under another
name is recognised as the same work, where the view only compared raw titles) —
and rewired the view to call it. One behavior note: an empty query searches
nothing, matching what the view already did.

### Two things that looked like dead code but were not

Resolved 2026-10-17 (commit `ec12590`): `AddGenre`/`RemoveGenre` really were
leftovers — both routes call `ToggleGenre`, which alone carries the final
behaviour; `EnrichmentCatalog` and `LoadOrInit` were leftovers too (both real
callers use `EnrichmentSources` / the `Default()`-then-`Load()` dance), so all
four were deleted. Follow-up: the two `ToggleGenre` handlers themselves were
byte-identical except their log text and collapsed into one
`handleToggleGenre` serving both routes.

## A real bug, not dead code — fixed in `df349e5`

`workers.Permanent()` (`internal/workers/pool.go:99`) wraps an error in
`ErrPermanent`, and `internal/workers/enqueue.go:238` checks for it to break
out of the retry loop — but **nothing in production code ever called
`Permanent()`**; only `pool_test.go:151` did. `df349e5` added the producer:
`permanentIfFinal` in `jobs.go` wraps exactly `*hostnet.ChallengeError` (needs a
human to open VerifyURL, so retrying only hammers the site) on the two
retryable seams, `runOnFetch` and `EnqueueExportCBZ`. Covered by
`TestRunOnFetchDoesNotRetryAChallenge` (1 attempt) and
`TestRunOnFetchRetriesATransientError` (3 attempts).

## Merge candidates — all six applied

Ordered by payoff. Each was verified by diff, not by eyeball.

1. ✅ **`handleAddGenre` / `handleRemoveGenre`** — collapsed into one
   `handleToggleGenre` in `internal/httpserver/actions_enrichment.go`; both
   routes in `actions.go` point at it (bodies were identical bar the log text).
2. ✅ **`EnqueueSearchManga` / `EnqueueGetChapterList`** — both are now thin
   wrappers over one `enqueueFanout[T]` in `internal/bridge/jobs.go` (fetch
   lane + plugin key + error handed to the callback, job returns nil either
   way). Public signatures unchanged, so no caller moved.
3. ✅ **`migrate_candidates.go` vs `views_migrate.go`** — resolved the other
   way: the view's inline copy was deleted and it calls
   `CollectMigrationCandidates` (see "wired, not deleted" above).
4. ✅ **`NormalizeTitle` in three layers** — `database.NormalizeTitle` deleted
   (`df349e5`), then the middle layer `database.normalizeTitle` deleted in the
   merge session: `duplicates_find.go` (4 sites) and `duplicates_synopsis.go`
   (1 site) call `pluginutil.NormalizeTitle` directly.
5. ✅ **`resolve-id → db → wrap` methods** — `withManga`/`withChapter` added to
   `internal/bridge/service.go` (resolve labelled `bridge: resolve manga|
   chapter`, fn labelled `bridge: <stage>` — messages byte-identical to the
   inline originals). 11 sites converted: 8 in `progress_read.go` (its `fmt`
   import now gone) and 3 in `progress.go` (`SetChapterProgress`,
   `SaveReaderPrefs`, `SetChaptersSkip`). Deliberately **not** converted:
   multi-step methods with several distinct stage labels (`SetMainTitle`,
   `RemoveAltTitle`, `ToggleCoverDim`), value-returning ones
   (`GetReaderPrefs`, `GetChapterProgresses`, `ListAltSummaries`),
   `alttitles.go`'s `MangaRowID` (string row id, different resolver), and
   `ToggleLibraryItem` (intentionally discards the resolve error) — a
   single-stage helper would have changed their error shapes. One message
   nuance: `SetChapterTotalPages`'s db error is now wrapped
   `bridge: set chapter total pages` where it used to propagate raw; no test
   or caller matches on that text.
6. ✅ **`ImageFormat.FormatExtension()`** — deleted in `df349e5` (it was a
   one-line alias of `extension()`); `TestFormatExtension` still covers
   `extension()` itself.

## Still open

- **`AutoSelectCandidate`** (`internal/bridge/migrate_candidates.go`) — the
  single remaining `deadcode ./...` hit. Its helpers now run in production, but
  no UI path auto-picks a migration target yet. Wire it when the picker grows
  an auto-select affordance, or delete it with its tests if that never lands.

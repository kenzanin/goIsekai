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

Findings only — nothing in this document has been applied yet. Recorded so the
work can be picked up without re-running the sweep.

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
deadcode ./...          # 19 hits: reachable only from main
deadcode -test ./...    # 12 hits: dead even when tests are entry points
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

### Unreachable even from tests (12)

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

### Reachable only from tests (7)

`altTitlesFor`, `matchKeys`, `isExactMatch`, `AutoSelectCandidate` (all in
`internal/bridge/migrate_candidates.go`), `ImageFormat.FormatExtension`
(`internal/bridge/imgcodec.go:49`), `database.NormalizeTitle`
(`internal/database/duplicates_match.go:13`), `workers.Permanent`
(`internal/workers/pool.go:99`).

### The largest single block

`internal/bridge/migrate_candidates.go` is 198 lines with no production caller.
`internal/httpserver/views_migrate.go` re-implements the same parallel
per-plugin search inline (its own `futures` map, its own failure collection,
its own `errAllSourcesFailed`) instead of calling
`CollectMigrationCandidates`. Deleting the file removes the dead code and the
duplication together — but `migrate_test.go` and `migrate_candidates_test.go`
cover `AutoSelectCandidate`, so those tests go with it.

### Two things that look like dead code but are not

- `AddGenre`/`RemoveGenre` are unused because **both** `handleAddGenre` and
  `handleRemoveGenre` call `ToggleGenre` — the two routes are behaviourally
  identical (they are byte-identical apart from the function name and two log
  strings). Whether that is intended has not been confirmed.
- `EnrichmentCatalog`, `LoadOrInit`, and `FormatExtension` read as deliberate
  exported API surface rather than leftovers. Confirm before deleting.

## A real bug, not dead code

`workers.Permanent()` (`internal/workers/pool.go:99`) wraps an error in
`ErrPermanent`, and `internal/workers/enqueue.go:238` checks for it to break
out of the retry loop — but **nothing in production code ever calls
`Permanent()`**. Only `pool_test.go:151` does. So the documented "do not retry
rate-limit-exhausted / permanent 404" path is wired at one end only;
production callbacks wrap their errors with plain `fmt.Errorf`, and those jobs
retry to `MaxAttempts` as normal.

## Merge candidates

Ordered by payoff. Each was verified by diff, not by eyeball.

1. **`handleAddGenre` / `handleRemoveGenre`** —
   `internal/httpserver/actions_enrichment.go:36` and `:118`. Identical body;
   both delegate to `ToggleGenre`. Collapse to one handler and point both
   routes at it.
2. **`EnqueueSearchManga` / `EnqueueGetChapterList`** —
   `internal/bridge/jobs.go:131` and `:148`. Differ only in which bridge method
   they delegate to (`SearchManga` / `GetChapterList`); `Lane`, `PluginKey`,
   and the callback plumbing are verbatim. One generic enqueue helper fits both.
3. **`migrate_candidates.go` vs `views_migrate.go`** — see above. Delete the
   helper, and the duplication goes with the dead code.
4. **`NormalizeTitle` in three layers** — `pluginutil.NormalizeTitle`
   (the real implementation), `database.normalizeTitle` (6 callers, a bare
   pass-through), and `database.NormalizeTitle` (0 non-test callers, another
   pass-through of the same thing). The last one is pure noise; the middle one
   could be an alias.
5. **31 near-identical `resolve-id → db → wrap` methods** across
   `internal/bridge/progress.go` (11), `progress_read.go` (10),
   `alttitles.go` (5), `library_catalog.go` (5). Each is 7–13 lines doing
   `ResolveMangaIntID`/`ResolveChapterIntID`, one `s.db.*` call, and a
   `fmt.Errorf` wrap. The `SIMILAR_TO` graph puts most pairs at jaccard ≥ 0.95.
   A `withManga`/`withChapter` helper collapses the resolve+wrap boilerplate.
6. **`ImageFormat.FormatExtension()`** — `internal/bridge/imgcodec.go:49` is a
   one-line alias of `extension()`. Inline it (or unexport `extension`).

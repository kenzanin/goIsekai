---
type: Concept
title: Dead code and merge candidates audit (2026-10-17)
description: "Sweep of 2026-10-17: 19 unreachable functions, unused migrate_candidates.go, six merge candidates, and a real Permanent() bug (never called in prod) fixed in df349e5."
tags: [goisekai, audit, cleanup, dead-code, refactor]
generated: { by: agent/mcp, at: "2026-10-26T15:59:35Z" }
status: stable
---

# Dead code and merge candidates audit (2026-10-17)

Sweep of 2026-10-17. Status: everything actionable applied across ec12590, df349e5 - 19 to 1 dead functions, six merge candidates all closed except AutoSelectCandidate.

## How to reproduce

deadcode + staticcheck in ~/go/bin were built with go1.26 and cannot parse this go1.27 module. Rebuild first:
```bash
go install golang.org/x/tools/cmd/deadcode@latest
go install honnef.co/go/tools/cmd/staticcheck@latest
deadcode ./...          # 1 hit: AutoSelectCandidate (test-only, open finding)
deadcode -test ./...    # 0 hits (was 19/12 at sweep time)
```
SIMILAR_TO edge (jaccard score) covers structural questions. CONTAINS and NOT ENDS WITH returned zero rows; STARTS WITH works.

## Dead code (resolved)

Unreachable even from tests (12) - 10 removed in ec12590: EnrichmentCatalog, GetMangaDetailsCached, SetMangaGenres, AddGenre, RemoveGenre, ClearPluginLogo, CollectMigrationCandidates, collectCandidatesWithTitle, RecordRead, Log, LoadOrInit, LuaEngine.Has. Wired in df349e5: CollectMigrationCandidates/collectCandidatesWithTitle (view now calls them).

Reachable only from tests (7) - all resolved: altTitlesFor, matchKeys, isExactMatch (production via df349e5), workers.Permanent (permanentIfFinal added), ImageFormat.FormatExtension + database.NormalizeTitle (deleted in df349e5).

migrate_candidates.go (198 lines, no production caller) - kept + wired into migrate view (better alt-title matching). Two-step: deleted inline copy in views_migrate.go, calls CollectMigrationCandidates.

## A real bug, not dead code - fixed in df349e5

workers.Permanent() wraps error in ErrPermanent; enqueue.go checks for it to break retry loop - but NOTHING in production ever called Permanent(); only pool_test.go. df349e5 added producer: permanentIfFinal in jobs.go wraps exactly *hostnet.ChallengeError on runOnFetch and EnqueueExportCBZ. Covered by TestRunOnFetchDoesNotRetryAChallenge (1 attempt) + TestRunOnFetchRetriesATransientError (3 attempts).

## Merge candidates - all six applied

1. handleAddGenre/handleRemoveGenre -> handleToggleGenre.
2. EnqueueSearchManga/EnqueueGetChapterList -> wrappers over enqueueFanout[T].
3. migrate_candidates.go vs views_migrate.go -> inline deleted, calls CollectMigrationCandidates.
4. NormalizeTitle three layers -> database.NormalizeTitle deleted, middle layer deleted, callers use pluginutil.NormalizeTitle directly.
5. resolve-id -> db -> wrap -> withManga/withChapter in service.go (11 sites).
6. ImageFormat.FormatExtension() -> deleted (one-line alias).

## Still open

AutoSelectCandidate - single remaining deadcode hit. Helpers run in production but no UI path auto-picks. Wire when picker grows auto-select, or delete with tests.

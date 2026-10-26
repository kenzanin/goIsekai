---
type: Concept
title: library_fts index sync and auto-heal
description: "FTS5 library_fts sync choke points, CAST(manga_row_id AS TEXT) key-comparison fix, LibraryFTSDrift/EnsureLibraryFTS auto-heal, scalability ceiling."
tags: [goisekai, database, fts5, search, maintenance, finding]
generated: { by: agent/mcp, at: "2026-10-26T15:26:22Z" }
status: stable
---

# library_fts index sync and auto-heal

(source: EchoesVault page `library-fts-index-sync.md`)

## Schema quirks (root cause of most bugs)

- `library_fts` is FTS5 with `manga_row_id UNINDEXED` — **no column affinity**. A bound string id (`'1'`) never matches an INTEGER key (`1`) and vice versa in DELETE/UPDATE. `mangas.id = ?` matches strings via INTEGER affinity — that asymmetry made INSERTs work while DELETE/UPDATE silently no-oped.
- Fix: all FTS key comparisons use `CAST(manga_row_id AS TEXT) = ?` (or CAST both sides).
- `libraryFTSIndex` (alttitles.go) is the single insert: `title` from `mangas`, `alt` = `group_concat(alt_titles.title, ' ')`.
  `SyncFTS(rowID string)` = delete + re-index one row; exported `SyncFTSTx(tx, rowID)` serves transactional callers (source migration).
- `alt_titles.manga_row_id` is TEXT-affinity and stores string keys; joins against INTEGER `mangas.id` work via cross-affinity numeric conversion.

## Mutation choke points (all now self-syncing)

- `UpsertManga` — insert + title refresh (covers `persistMangaDetails`, `RefetchCover`)
- `ToggleLibrary` — membership flip (was THE 299-vs-88 gap creator)
- `RepointManga` — plugin/source/title change
- `AddAltTitles` / `RemoveAltTitle` — alt titles (covers `storeEnrichment`)
- `ResetEnrichment` — alt wipe
- `SwapMainTitle` — inline UPDATE (fixed CAST)
- `MigrateMangaSource` — now calls `database.SyncFTSTx` (old local copy used wrong column `alt_titles` and was silently swallowed)

## Auto-heal

`DB.LibraryFTSDrift()` — one query covering missing rows, dangling/duplicate rows (count), and stale title/alt content. `DB.EnsureLibraryFTS()` rebuilds (`RebuildLibraryFTS`) only on drift. Wired in `cmd/goisekai/maintenance.go` `startMaintenance`: at startup + every maintenance tick, unconditional (search correctness, not gated on `PruneOrphans` config). `PruneOrphans` gained `orphan_fts` (dangling FTS rows for deleted mangas).

## Gotchas

- `internal/bridge/migrate.go:66 type candResult` is pre-existing unused (on HEAD, zero refs) — left untouched.
- FTS5 DELETE/UPDATE on unindexed columns is a table scan — fine at ~300 rows; if the library grows to 10k+, switch the key to always-INTEGER and pass int64.

# Design: optimize-db-storage

## Context

See proposal.md — Why. The database currently carries 16,359 `chapters` rows (10,453 for `in_library=0` manga), a 1.7 MB string-composite autoindex on `chapters.id` (`<plugin>|<manga-slug>|<chapter-id>`, with the manga slug stored twice per row), 5,275 `read_history` rows where every consumer only reads the latest per chapter, and 2.4 MB of `chapter_pages` JSON blobs that are re-fetchable. The owner has accepted data loss for these classes.

Constraints: migrations are ordered strings applied via `PRAGMA user_version` (`internal/database/schema.go`); queries go through go-jet with generated tables in `.gen/`; pure-Go SQLite (no CGO); WAL mode; the startup janitor `PruneOrphans()` in `internal/database/maintenance.go` already exists as the natural home for recurring purges.

## Goals / Non-Goals

**Goals:**

- Shrink the on-disk library from ~9.6 MB to roughly 2 MB or less and keep it small over time.
- Make chapter-list queries index lookups (`manga_id`) instead of scans over all manga's chapters.
- Keep externally observable behavior identical (library, history page, reader progress).
- One migration that old databases run through automatically.

**Non-Goals:**

- Moving `chapter_pages` blobs out of SQLite onto the disk image-cache path (they are small enough once pruned; revisit only if page-cache rows grow).
- Compressing or trimming `mangas.description` / alt-title text (durable data, not cache).
- Any UI for managing storage or showing DB size.

## Decisions

### D1: integer surrogate keys for chapters, string keys stay for mangas
`chapters.id INTEGER PRIMARY KEY`, `UNIQUE(manga_id, source_chapter_id)`, `INDEX chapters(manga_id)`. `chapter_pages.chapter_id` and `read_history.chapter_id` become integer FKs. `mangas` keeps its `plugin|slug` text id — mangas are few (157), their key size is irrelevant, and rewriting them would touch every handler path for no storage win.
Alternative considered: keep string keys and just add the `manga_id` index. Rejected: it keeps paying the 1.7 MB autoindex and the double-slug row bloat forever; the migration is already destructive, so the one-time rewrite is effectively free to piggyback.

### D2: mapping source ids to surrogates via `INSERT ... ON CONFLICT ... RETURNING id`
`persistMangaDetails` upserts each chapter with `ON CONFLICT(manga_id, source_chapter_id) DO UPDATE` returning the integer id, so callers that need the chapter id (page caching, progress) get it back in the same statement. No read-back queries, no `chapterRowID()` string builder — it is deleted.
Alternative: a separate `chapter_ids` lookup table. Rejected: one more join for no benefit.

### D3: purge policy lives in `PruneOrphans`, on-demand persistence at the call site
Two halves:
1. `GetMangaDetails`/`persistMangaDetails` skips chapter persistence when the manga is not `in_library` (chapters are fetched live for the detail view anyway; they just stop being written). Toggling a manga into the library re-persists chapters on the next detail fetch/sync.
2. `PruneOrphans()` gains three statements: delete chapters of `in_library=0` manga (cascade carries pages/history), delete duplicate `read_history` rows keeping the newest per chapter, delete `chapter_pages` rows whose chapter `is_read=1`.
Alternative: purge inside the library-toggle action. Rejected: the janitor already runs at startup and periodically, and legacy rows need a sweeper regardless of which path created them.

### D4: `read_history` becomes upsert-per-chapter
`RecordRead` switches from INSERT to `INSERT ... ON CONFLICT(chapter_id) DO UPDATE SET page_num, read_at=CURRENT_TIMESTAMP`, with a unique index on `chapter_id`. Consumers (`GetReadHistory` via `MAX(read_at)`, progress lookups) already read latest-per-chapter semantics, so no query changes.
Alternative: keep append log and dedupe in queries. Rejected: unbounded growth for zero benefit.

### D5: schema rebuild for the key change + VACUUM in the same migration
SQLite cannot retype a primary key in place, so migration N does the classic rename-and-copy dance: create new `chapters`/`chapter_pages`/`read_history` (and the unique/index), copy only rows worth keeping (all of `chapters` that survive the purge; newest history row per chapter; no page rows for read chapters), drop old tables, rename. Then the D3 purge statements run (for rows the copy already filtered this is a no-op), and `VACUUM` reclaims the file. go-jet `.gen/` tables are regenerated afterwards; query files switch `ChapterID` types from string to int64.
Rollback: none beyond restoring a backup from `<data_dir>/backups/` — the periodic backup runs before migrations at startup, which is accepted given the owner's data-loss waiver.

### D6: keep `plugins`/`alt_*`/`manga_categories`/`manga_related` untouched
They are small (13/621/112/285/255 rows) and durable. No win worth churn.

## Risks / Trade-offs

- [Migration bug corrupts live libraries] → The DDL runs inside one transaction per migration step; backups exist in `<data_dir>/backups/` from the periodic job; the owner accepted data loss explicitly, but code still fails closed: a failed migration aborts startup rather than half-applying.
- [Chapter list of a non-library manga now hits the plugin every open] → Intended trade-off: search-browsing is ephemeral; adding to the library restores persistence on next open.
- [`is_read`-based page-cache pruning evicts a cache row mid-reading] → Pruning only targets `is_read=1` (fully read), and eviction is benign by construction: the reader re-fetches page lists transparently.
- [Upsert-on-read changes history row ids under readers' feet] → Nothing references `read_history.id` externally; `GetReadHistory` orders by `MAX(read_at)` + `MAX(id)`, which stays well-defined.
- [Wide blast radius: every `chapterRowID` caller] → `grep -rn chapterRowID` enumerates them; the compiler enforces the int64 switch, so nothing can be silently missed.

## Migration Plan

1. Ship as one new entry in the `migrations` slice (bumps `user_version` once).
2. On next start: backup (existing behavior) → migration runs D5 copy + D3 purges → `VACUUM` → `PruneOrphans` no-ops.
3. Verify on the dev copy: row counts (`chapters` ≈ in-library count, `read_history` ≤ chapter count, `chapter_pages` pruned) and file size; then run `just test`.
4. Rollback strategy: restore the pre-migration backup file; old binary + old schema is the only compatible pairing.

## Open Questions

- Should the periodic janitor interval (if any, beyond startup) run the new purges, or startup-only? Answerable when wiring `PruneOrphans` call sites; does not affect schema or specs.

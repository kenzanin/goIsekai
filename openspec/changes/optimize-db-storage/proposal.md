# Proposal: optimize-db-storage

## Why

The SQLite library has grown to 9.6 MB (+4.1 MB WAL) for 157 manga, of which 64% of `chapters` rows belong to manga that were merely viewed from search results (`in_library=0`), 5,275 `read_history` rows record one event per page turn when every reader of that table only ever needs the latest row per chapter, and the string-composite primary keys (`<plugin>|<manga-slug>|<chapter-id>`) store the full manga slug twice per chapter row (column + autoindex) while `chapters` has no index on `manga_id`, making chapter lists a full scan over 16K rows. Data loss is acceptable per the owner, so the fat can be cut rather than tolerated.

## What Changes

- Purge policy: chapters (and cascading `chapter_pages` / `read_history`) of manga with `in_library=0` are deleted, and detail fetches for manga not in the library stop persisting chapters. Chapters load on demand when the manga is opened.
- Read history is deduplicated to the latest row per chapter (upsert semantics on `(chapter_id)`); the history page and reading progress behave as before because they only aggregate `MAX(read_at)`.
- `chapter_pages` is treated as a cache: rows for chapters fully read are prunable, and the whole table is emptied by the one-time compaction.
- One-time compaction on migration: purge the above, then `VACUUM` so the file actually shrinks.
- **BREAKING (schema)**: `chapters.id` becomes an `INTEGER PRIMARY KEY` with `UNIQUE(manga_id, source_chapter_id)`; `chapter_pages.chapter_id` and `read_history.chapter_id` reference the integer key; `chapters.manga_id` becomes an integer FK to `mangas`' row id (mangas keep their `plugin|slug` text id); an index on `chapters(manga_id)` is added.

## Capabilities

### New Capabilities

- `db-maintenance`: background data hygiene for the SQLite library — non-library chapter purging, read-history deduplication, page-cache pruning, and post-migration `VACUUM` compaction, with the existing `PruneOrphans` startup janitor extended to carry them out.

### Modified Capabilities

- `storage`: chapter rows are keyed by an integer surrogate instead of the string composite key, `manga_id` becomes an integer reference with an index, `read_history` keeps at most one row per chapter (latest wins), and `chapter_pages` is specified as a re-fetchable cache rather than durable data. The externally observable library/reader behavior does not change.

## Impact

- `internal/database/schema.go` (new migration), `migrations` slice grows by one.
- `internal/database/chapters_query.go`, `history.go`, `maintenance.go`, `mangas.go` and the go-jet `.gen/` tables (regenerate).
- `internal/bridge/library.go`, `sync.go` (`chapterRowID` removal; callers switch to integer ids).
- `internal/httpserver` handlers that build chapter ids from path segments.
- Existing installs migrate on first run; the migration is destructive for non-library chapters, stale read-history detail, and the page cache — accepted.

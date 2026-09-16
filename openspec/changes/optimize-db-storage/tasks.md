## 1. Schema migration

- [x] 1.1 Add migration N to `internal/database/schema.go`: create new `chapters` (integer surrogate `id`, integer `manga_id` FK, `UNIQUE(manga_id, source_chapter_id)`, `INDEX chapters(manga_id)`), new `chapter_pages` and `read_history` with integer `chapter_id` + `UNIQUE(chapter_id)` on read_history; copy only surviving rows (skip chapters of `in_library=0` manga, newest read_history row per chapter, drop `chapter_pages` of read chapters); drop old tables; rename. Verify with a test that migrates a fixture DB built on schema N-1 and asserts row counts and `user_version`.
- [x] 1.2 End the migration with `VACUUM` and verify in the same test that a DB padded with deleted rows shrinks after migration.
- [x] 1.3 Regenerate go-jet tables (done) (`.gen/`) for the new schema and verify `go build ./...` passes.

## 2. Data-access layer

- [x] 2.1 Rework `internal/database/chapters_query.go` (and `chapters.go`, `history.go`, `mangas.go` as needed) for integer chapter ids: upsert via `ON CONFLICT(manga_id, source_chapter_id) DO UPDATE ... RETURNING id`; all ChapterID params/returns become int64. Verify `go test ./internal/database/...`.
- [x] 2.2 Switch `RecordRead` to upsert-on-chapter (needs test) (`ON CONFLICT(chapter_id) DO UPDATE SET page_num, read_at`). Add a test: record the same chapter twice, assert one row with the latest page/timestamp.
- [x] 2.3 Delete `chapterRowID` in `internal/bridge/sync.go` (bridge layer needs updates) and update its callers (`library.go`, httpserver handlers building chapter ids from path segments) to carry integer ids from the DB layer. Verify with `go build ./...` (compiler pins every site).

## 3. Purge policy

- [x] 3.1 Skip chapter persistence in `persistMangaDetails` when the manga is not `in_library`; verify with a bridge test that fetching details for a non-library manga leaves `chapters` empty while the detail view still returns chapters.
- [x] 3.2 Extend `PruneOrphans` in `internal/database/maintenance.go` with: chapters of `in_library=0` manga, duplicate read_history rows (newest kept), `chapter_pages` of `is_read=1` chapters. Add tests for each purge keeping their negatives (in-library chapters, single history rows, unread chapters' pages survive).
- [x] 3.3 Verify reading flow end-to-end: mark-read + read a page + re-open a pruned chapter re-fetches its page list (bridge or httpserver test).

## 4. Verification and docs

- [x] 4.1 Full gate: `just test`, `just race`, `just check` all green.
- [x] 4.2 Migrate a copy of the real dev DB (`cp` + run binary once), assert: `chapters` count ≈ in-library chapters, `read_history` ≤ chapters, file size dropped (~9.6 MB → ~2 MB or less), reader and library pages still work in the browser.
- [x] 4.3 Update `openspec/specs/storage/spec.md` requirement headers only if wording drifts during implementation (delta specs already carry the change; this is a sweep) — verify `openspec validate optimize-db-storage --strict` passes.

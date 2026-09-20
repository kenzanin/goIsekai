## Why

A library entry is pinned to the plugin it was first added from. When that source
degrades, dies, or simply offers a worse copy, the only recovery today is to
delete the entry and re-add the title elsewhere, which throws away read progress
and forces the reader to re-mark chapters by hand. Suwayomi lets a title move to
another source while keeping its reading state; goIsekai has no equivalent.

## What Changes

- The manga detail page gains a migrate action: every active plugin that is not
  info-only is searched for the same title and the results become candidates.
- A candidate whose normalized title equals the library title exactly is selected
  automatically; otherwise the candidates are listed for the reader to pick.
- Applying a migration repoints the existing manga row at the target source by
  rewriting its `plugin_id` and `source_manga_id`. The row keeps its `id`, so
  alt titles, categories, related entries and the library FTS row stay attached
  and the title never appears twice in the library.
- The target source's chapters replace the old chapter list, and read state
  carries over: a new chapter is marked read when its `chapter_num` is at or
  below the highest chapter number that was read on the old source.
- **BREAKING** (data loss, deliberate): once the target chapters are written, the
  old source's chapter rows are deleted. Their `chapter_pages` rows (downloaded
  pages) and `read_history` rows cascade away with them, and mid-chapter
  `last_page_read` position is not carried over. Read/unread state is the only
  progress that survives a migration.
- Migration refuses to run when the target manga already exists as a separate
  library row, because repointing would violate the unique constraint on
  `(plugin_id, source_manga_id)`.
- Migration is a manual, reader-triggered action. Nothing migrates in the
  background, and a failed migration must leave the entry on its original source.

## Capabilities

### New Capabilities
- `manga-source-migration`: finding another source for a title already in the
  library, choosing which source to move to, and repointing the entry at it while
  carrying read/unread state.

### Modified Capabilities
- None. Chapter and manga persistence keep their existing contract; the
  migration operation is what changes, not the storage rules.

## Impact

- `internal/database/`: new query builders to rewrite a manga's source identity
  and to prune the chapters that no longer belong to it. Reuses `UpsertChapter`,
  `SetChaptersUpTo` and `SyncFTS`, which already do the chapter write, the
  read-up-to rule and the FTS rebuild.
- `internal/bridge/`: new `MigrateMangaSource` orchestration and a candidate
  lookup layered on the existing `SearchManga`. No new plugin ABI surface.
- `internal/httpserver/`: a new `POST /action/migrate-source/{pluginID}/{mangaID}`
  action alongside the other library mutations, plus candidate data handed to the
  detail view.
- `internal/templates/views/detail.lua`: candidate list and the apply control.
- `plugin-abi` is untouched: `Search` and `GetChapterList` already return
  everything the migration needs.
- Reader-visible consequence: a migrated title loses downloaded pages and read
  history for the old source's chapters. This is intentional and must be stated
  in the UI before the reader confirms.

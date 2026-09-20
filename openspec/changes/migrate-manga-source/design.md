## Context

See `proposal.md` for the motivation. The constraints that shape the approach:

- **Enrichment cannot find a source.** `internal/enrich/` searches a fixed set of
  metadata providers declared by info scripts, and it never returns a source
  manga id. Discovery has to go through the scraper plugins themselves:
  `AppService.SearchManga(pluginID, types.SearchFilter{Query: title})`, which
  wraps the plugin `Search` export. `types.SearchFilter` carries only `Query`,
  `Page`, `Genres`, `SortBy`, and `types.Manga` carries no similarity score, so
  the host must do its own ranking.
- **Chapters have no cross-source identity.** `chapters` is unique on
  `(manga_id, source_chapter_id)`, and `source_chapter_id` is meaningless on any
  other source. `chapter_num` (a float) is the only comparable quantity.
- **The read-up-to rule already exists.** `SetChaptersUpTo(mangaID, sourceIDs,
  read)` marks every chapter of a manga whose `chapter_num` is at or below the
  highest `chapter_num` among the given source chapters. It derives that bound by
  reading the existing chapter rows of that manga.
- **`UpsertChapter` preserves state.** Its `ON CONFLICT (manga_id,
  source_chapter_id) DO UPDATE` refreshes title and numbering but deliberately
  leaves `is_read`, `last_page_read` and `download_status` alone, so re-fetching a
  known chapter never clobbers progress.
- **`mangas.id` is the anchor.** `alt_titles`, `alt_descriptions` and
  `manga_categories` key on `manga_row_id` = `mangas.id`, as does the
  `library_fts` document. `library_fts` has no triggers and stores `plugin_id`
  as an unindexed column, so a source change must be re-indexed by hand.
  `SyncFTS(mangaRowID)` already removes and re-inserts one document.
- **The destructive cascade.** `chapters` cascades from the manga row, and
  `chapter_pages` and `read_history` cascade from the chapter row. Deleting a
  chapter takes its downloaded pages and read history with it.
- **Title matching exists but is local.** `FindPotentialDuplicates` and
  `normalizeTitle` compare rows already in the database. `normalizeTitle` is
  reusable as the project's definition of "same title"; the duplicate detector
  itself cannot reach a plugin.

## Goals / Non-Goals

**Goals:**

- Repoint a library entry at another source while keeping the entry's row id, so
  enrichment and library search survive untouched and the title never appears
  twice.
- Reduce the migration to existing primitives wherever possible, so the new code
  is orchestration plus one statement that has no precedent (rewriting a source
  identity).
- Make the irreversible part of the operation structurally impossible to reach
  unless the recoverable parts are already committed.

**Non-Goals:**

- No background or bulk migration, and no automatic replacement of a plugin that
  has gone dark.
- No cross-source chapter identity, and no per-chapter fuzzy title matching.
- No transfer of downloaded pages or read history.
- No merging of two existing library entries.
- No new plugin ABI surface and no new dependency: `Search`, `GetMangaDetail`
  and `GetChapterList` already return everything needed.

## Decisions

### Repoint the existing row rather than create a new entry

Rewriting `plugin_id` and `source_manga_id` on the existing `mangas` row keeps
every child keyed on `mangas.id` intact: alt titles, alt descriptions,
categories, and the FTS document. The library also never shows the title twice,
and no "which one is the real entry" state exists to clean up.

The alternative, a Suwayomi-style new entry under the target source with an
optional delete of the old one, was considered and rejected for this codebase:
the title would be duplicated until the reader deletes the old row, and that
delete destroys exactly the enrichment and progress the new row cannot inherit.

Consequence to remember: because the row id is stable, `library_fts` needs its
`plugin_id` refreshed in place rather than a new document.

### Write the new identity with a dedicated statement, not `UpsertManga`

`UpsertManga` resolves on `(plugin_id, source_manga_id)`. Handing it the target's
pair would insert a second row instead of repointing the first. The migration
needs an explicit update of `plugin_id`, `source_manga_id` and the refreshed
title, cover, description and status on a known row id, applying the same
`custom_title` / `custom_description` guards `UpsertManga` applies so a
hand-edited title is not overwritten.

### Transfer read state through the existing `SetChaptersUpTo`

The spec's rule is "read through the highest chapter read on the old source".
`SetChaptersUpTo` already implements exactly that, given the old source's read
chapter ids. It resolves the bound from the chapter rows of the manga, so the old
rows must still be present when it runs. That fixes the ordering inside the
transaction:

1. repoint the manga row at the target source
2. insert the target source's chapters
3. apply the read boundary from the old source's read chapter ids
4. delete the chapters that are not the target's
5. re-index the FTS document

Writing a second bulk-update comparing `chapter_num` between two sources was
considered and rejected as a duplicate of `SetChaptersUpTo`.

### One transaction, with the destructive delete last

The target's detail and chapter list are fetched from the plugin before the
transaction opens, so no write lock is held across a network call that may take
up to the 15 second invoke budget.

The migration then runs as a single write transaction whose final mutation is the
delete of the old chapters. Because that delete is what cascades `chapter_pages`
and `read_history` away, ordering it last means a failure anywhere earlier leaves
the entry untouched: the transaction rolls back and the old chapters, pages and
history are still there. The read flags and target chapters are committed by the
same transaction that removes the old rows, so there is no window in which the
entry has been repointed but still has no chapters.

### Rank candidates by normalized title, decided by the host

Every active, non-info plugin is searched with the display title.
`normalizeTitle` (the duplicates subsystem's definition of "same title") decides
whether a candidate is an exact match. Exactly one exact match is selected
automatically; zero or several exact matches are presented for the reader to
choose. The display-title pass falls back to the entry's alternative titles only
when it produced nothing.

`FindPotentialDuplicates` was considered as the discovery mechanism and rejected:
it compares rows already in the database, so it can never reveal a source the
library does not already know about.

### Search lazily, on the detail view only

The candidate search costs one plugin invocation per active source, each under
the 15 second invoke budget, so it must never run while a list is being rendered.
It runs only for the manga detail view, on the reader's request. This mirrors the
existing decision to skip `FindPotentialDuplicates` when the library is filtered.

### Refuse before writing when the target is already a library entry

If the target pair already exists as its own library row, the migration is
refused with a named conflict. Otherwise `UNIQUE(plugin_id, source_manga_id)`
would abort the transaction and surface a constraint error instead of an
explanation. Merging, overwriting or deleting either entry is out of scope.

### Confirm through the existing action convention

The apply control is a form post to a new `/action/migrate-source/...` route
registered beside the other library mutations, answering with a 303 redirect,
and the unrecoverable-loss warning is part of the control that submits it. No new
client-side mechanism is introduced.

## Risks / Trade-offs

- **Deleting the old chapters discards downloaded pages and read history** → the
  loss is disclosed before the control that submits it, and the delete is the
  final statement of the transaction so it cannot precede a successful repoint.
- **Mid-chapter position is lost** → accepted by product decision; only
  read/unread state is promised, and the spec says so.
- **Sources that number by volume or use `.5` chapters shift the boundary** →
  disclosed rather than solved; the derived flags land in the chapter list the
  reader already manages with the existing chapter actions.
- **N plugin invocations on one page** → the search is reader-triggered and
  confined to the detail view.
- **Enrichment on a repointed entry came from the previous source** → left in
  place, since it describes the story rather than the source; the reset
  enrichment action already exists if the reader wants it re-derived.
- **Stale `plugin_id` in `library_fts` would break filtered search** → `SyncFTS`
  runs inside the same transaction.
- **The old source URL in a bookmark or external link stops resolving** → the old
  `source_manga_id` is not retained anywhere after a migration; accepted.
- **The updates feed and new badge treat the target's chapter list as new
  input** → accepted; the existing clear-new action dismisses it.

## Migration Plan

No schema change and no data migration: this is a new operation over existing
tables. The destructive step is unrecoverable by design, so the safety net is the
periodic database backup under `<data_dir>/backups/` rather than an application
rollback. Deployment is a normal build; reverting the code returns the reader to
the previous behaviour with no schema cleanup.

## Open Questions

- Whether a migrated entry should also discard the enrichment that came from the
  previous source. Deferrable: the reset enrichment action already covers it.
- Whether the duplicate-groups panel should offer migration as a second entry
  point. Deferrable: it is another way to reach the same operation and changes no
  behaviour.

## 1. Database

- [x] 1.1 Add a method in `internal/database/mangas.go` that rewrites one manga's
      source identity by row id: set `plugin_id`, `source_manga_id` plus the
      refreshed title, cover, description and status, keeping `custom_title` and
      `custom_description` authoritative the same way `UpsertManga` does. Verify
      with a test that the row id is unchanged and that the `alt_titles` and
      `manga_categories` rows for that id still resolve afterwards.
- [x] 1.2 Add a method that deletes a manga's chapters whose `source_chapter_id`
      is not in a given set and returns the number removed. Verify with a test
      that a removed chapter takes its `chapter_pages` row and its
      `read_history` row with it, and that chapters of another manga are
      untouched.

## 2. Candidate discovery

- [x] 2.1 Export the duplicates subsystem's `normalizeTitle`
      (`internal/database/duplicates_match.go`) so the bridge can rank
      candidates with the project's existing definition of "same title". Verify
      the existing duplicates tests still pass unchanged.
- [x] 2.2 Add a bridge method that collects migration candidates for
      `(pluginID, mangaID)`: iterate `ListPlugins()` keeping only active entries
      and skipping the entry's own plugin, search each with
      `SearchManga(candidate, types.SearchFilter{Query: title})`, and keep
      every result as a candidate (no title filtering — equality only feeds
      2.4's selection rule). Collect per plugin: a source that errors, or
      returns zero results with nil error, is skipped and its failure reported;
      the batch proceeds. Verify with tests that an inactive plugin and the
      current plugin never appear among the candidates, that a failing plugin
      does not hide another plugin's results, and that all-plugins-failed is
      reported as unreachable rather than as no candidates.
- [x] 2.3 Add the alternative-title fallback: when the display-title pass yields
      no candidate, retry with each alternative title from `ListAltTitles`.
      Verify with a test where the entry's display title matches nothing and an
      alternative title matches.

## 3. Migration orchestration

- [x] 3.1 Add `MigrateMangaSource(oldPluginID, oldMangaID, newPluginID,
      newMangaID string) error` to `internal/bridge`, fetching the target's
      detail and chapter list before the transaction opens. Verify with a test
      that a successful call leaves exactly one manga row, carrying
      `(newPluginID, newMangaID)`, and none carrying the old pair.
- [x] 3.2 Refuse before writing when the target already exists: use
      `GetMangaCached(newPluginID, newMangaID)` and fail when it returns a
      non-zero id — the message names the target as already present in
      `mangas` (ghost rows with `in_library = 0` conflict too). Verify with a
      test that the conflict is reported and nothing is written.
- [x] 3.3 Implement the transaction body in the order given by `design.md`:
      repoint the manga row, insert the target chapters with `UpsertChapter`,
      apply the read boundary with `SetChaptersUpTo` using the old source's read
      chapter ids, delete the chapters that are not the target's, delete the
      old pair's `plugin_cache` rows, then `SyncFTS`. Verify with a test that
      the target chapters at or below the old read boundary end up read,
      chapters above it stay unread, `last_page_read` is not carried over, and
      no `plugin_cache` rows remain for the old pair.
- [x] 3.4 Verify failure atomicity: with the target fetch failing, and with a
      forced error partway through the transaction, the entry keeps its original
      `plugin_id`, its original chapters and its read flags.

## 4. HTTP and template

- [x] 4.1 Register `POST /action/migrate-source/{pluginID}/{mangaID}` in
      `internal/httpserver/actions.go` beside the other library mutations,
      reading the chosen target from the form and answering with a redirect.
      Verify the route is registered in `internal/httpserver/routes_test.go`.
- [x] 4.2 Hand the candidates and the automatic selection to
      `viewMangaDetail` and render one control per candidate plus the
      unrecoverable-loss warning in `internal/templates/views/detail.lua`.
      Verify by loading `/view/manga/{pluginID}/{mangaID}` and checking that the
      warning text and the candidate controls are present.
- [x] 4.3 Verify the apply control posts to the new action and that the detail
      view no longer offers the migration once the entry has been migrated.

## 5. Integration verification

- [x] 5.1 On a running server, migrate a library entry between two plugins that
      both carry the title, and verify against the database and the UI that the
      title appears once in `/view/library`, that read flags carried onto the
      target chapters, that the old chapters are gone, and that a library search
      for the title still returns the entry.
- [x] 5.2 Run `just check`, `just test`, `gofmt -l internal/ pkg/ cmd/`, and
      `luacheck internal/templates/ --codes --no-unused --no-unused-args`, and
      verify all are clean.

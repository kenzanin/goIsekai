# Tasks

## 1. Go splits — internal/httpserver (same-package moves + tests)

- [x] 1.1 Split `views_library.go` (247, single function): extract duplicate-group colour maps and the stats-map build into `libraryDupMaps`/`libraryStatsMap` in `views_library_data.go`; add unit tests for both helpers
- [x] 1.2 Split `actions_covers.go` (224): move `renderCoverModal` + `handleSaveReaderPrefs` to `covers_modal.go`; add table test for `renderCoverModal` headers (Content-Type, X-Toast)
- [x] 1.3 Split `views_manga.go` (222): move challenge/fallback branch helpers to `views_manga_fallback.go`; keep view entry in place
- [x] 1.4 Split `views_migrate.go` (212): move the inline per-plugin candidate search to `views_migrate_search.go`
- [x] 1.5 Split `api_reader.go` (200): move the progress/page handlers (`apiSetProgress`, `apiImage`) to `api_reader_pages.go`; pacing fallback coverage lives in task 2.1 (`bridge/image_fetch.go`)

## 2. Go splits — internal/bridge, internal/database, cmd

- [x] 2.1 Split `bridge/image_fetch.go` (234): move the retry/404 ladder to `image_fetch_retry.go`; keep existing `TestNotFoundIsNotRetried` green
- [x] 2.2 Split `bridge/sync.go` (216): move `syncMangas`/`SyncStaleLibrary` to `sync_batch.go`; add test for stale-cutoff selection with a fixture DB
- [x] 2.3 `database/schema.go` (214): evaluated — intentionally kept; ordered-migrations literal documented in the file header (commit 99a56f4)
- [x] 2.4 Split `cmd/goisekai/main.go` (213) and `cmd/goisekai/setup.go` (212): move flag/config wiring to `cmd/goisekai/config.go` if same-package move is possible; otherwise document as intentionally kept

## 3. Lua template splits (partials via existing require contract)

- [x] 3.1 Build the byte-equality render harness: test helper that renders a view with a rich fixture and compares against a golden string
- [x] 3.2 Split `views/detail.lua` (526): extract chapters block → `partials/detail_chapters` is already partial; extract info column + toolbar → `partials/detail_info.lua`; golden test passes
- [x] 3.3 Split `views/search.lua` (340): extract results grid + category filter → `partials/search_results.lua`, `partials/search_filters.lua`
- [x] 3.4 Split `views/logs.lua` (316): extract toolbar/filter bar → `partials/logs_toolbar.lua`
- [x] 3.5 Split `views/library.lua` (311): extract sort/tag toolbar → `partials/library_toolbar.lua`
- [x] 3.6 Split `views/migrate.lua` (257): extract candidate cards → `partials/migrate_candidates.lua`
- [x] 3.7 Split `partials/detail_chapters.lua` (232), `partials/library_sidebar.lua` (225), `partials/plugin_cards.lua` (222), `partials/detail_alt.lua` (207): extract row/card renderers into `*_row.lua` siblings
- [x] 3.8 Re-run all golden tests; confirm no template output changed

## 4. Unit-test completion for split Go units

- [x] 4.1 `views_library.go`: tests for `filterLibrary` each status branch (`reading`/`unread`/`done`/`new`) and tag filter case-insensitivity
- [x] 4.2 `actions_covers.go`: test set-cover happy path + error path returns 502; toggle-cover-dim returns `{"dim":n}` JSON
- [x] 4.3 `api_reader.go`: test reader-data 404 path and page-range clamping
- [x] 4.4 `bridge/image_fetch.go`: test retry budget exhaustion (non-404 persistent failure stops after budget)
- [x] 4.5 `bridge/sync.go`: test that inactive plugins are skipped and failures don't abort the batch

## 5. Final gate

- [x] 5.1 `scc` re-scan: every non-test source file in `internal/` + `cmd/` < 200 LOC (or explicitly marked intentionally kept in 2.3/2.4)
- [x] 5.2 `just check && just build && just test` green; restart server per deploy recipe; smoke test detail + library + search pages render identically
- [x] 5.3 Commit per-file splits (one commit per file) and the test-completion work separately

> 3.7 note: the splits themselves created two new partials still >200 LOC (`logs_script` 237, `plugin_card` 210); both are scheduled under 5.1 alongside the Go re-scan.

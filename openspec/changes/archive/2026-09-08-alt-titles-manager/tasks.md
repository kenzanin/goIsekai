# Tasks: alt-titles-manager

## 1. Schema & storage

- [x] 1.1 Add migration: create `alt_titles` table (`id INTEGER PK, manga_row_id → mangas(id) ON DELETE CASCADE, title TEXT, source TEXT, UNIQUE(manga_row_id, title)`), migrate the single populated `mangas.alt_titles` JSON row into it, drop `mangas.alt_titles`, create `library_fts` FTS5 table (`title, alt, plugin_id UNINDEXED, manga_row_id UNINDEXED`)
- [x] 1.2 Mirror DDL in `cmd/jetgen/main.go`, regenerate `.gen` model/table types (project rule: jetgen regen after column changes)
- [x] 1.3 DB layer (`internal/database/alttitles.go`): `AddAltTitles(mangaRowID string, titles []string, source string) (inserted int)`, `RemoveAltTitle(mangaRowID, title string) error`, `ListAltTitles(mangaRowID string) []AltTitleRow`, `SwapMainTitle(pluginID, sourceMangaID, newTitle string) error` (transactional swap, inserts old title with source `user`), `SearchLibraryFTS(q string) []CandidateRow`
- [x] 1.4 FTS sync funnel: update `library_fts` inside every write method that touches title/library membership/alt titles (manga upsert path, swap, add/remove, library toggle) + `rebuildLibraryFTS()` backfill helper called from migration

## 2. Plugin ABI

- [x] 2.1 `pkg/types`: add `AltTitleServer{ID, Name}` to `PluginMeta` (`alt_title_servers` JSON tag); `GetAltTitlesFunc` constant already exists
- [x] 2.2 `internal/pluginmanager`: `AltTitleServers()` — iterate discovered plugins, return aggregated `[]{ProviderPluginID, ServerID, Name}` from metadata (no instantiation of deferred plugins; metadata read at scan); keep `GetAltTitles(title, server)` dispatch (extend to pass server through)
- [x] 2.3 MangaDex JS plugin: add `alt_title_servers: [{id:"mangadex", name:"MangaDex"}]` to `PLUGIN` metadata; `getAltTitles(arg)` parses `{"title","server"}` and passes through; sync `examples/plugins/js/mangadex/main.js`

## 3. Bridge service

- [x] 3.1 `internal/bridge`: `AltTitleServers()`, `FetchAltTitles(pluginID, mangaID, server string) ([]AltTitleRow, error)` (resolve server→provider plugin, call, merge via `AddAltTitles`), `SetMainTitle(pluginID, mangaID, title string) error`, `RemoveAltTitle(pluginID, mangaID, title string) error`, `SearchLibrary(q string) ([]SearchHit, error)` (FTS candidates + Go fuzzy scoring: exact 100 / prefix 80 / substring 60 / subsequence 30, case+diacritic fold, cap 50)

## 4. API (api-first)

- [x] 4.1 `GET /api/alt-title-servers` — aggregated list, empty array when none
- [x] 4.2 `POST /api/manga/{pluginID}/{mangaID}/alt-titles` body `{"server"}` — fetch+merge, return merged list; 400 on unknown server
- [x] 4.3 `DELETE /api/manga/{pluginID}/{mangaID}/alt-titles` body `{"title"}` — 400 if title equals current main title
- [x] 4.4 `PUT /api/manga/{pluginID}/{mangaID}/title` body `{"title"}` — swap; 400 if title not in stored list
- [x] 4.5 `GET /api/library/search?q=` — ranked hits
- [x] 4.6 Extend manga detail API payload with `alt_titles: [{title, source}]`

## 5. UI

- [x] 5.1 Detail page: provider/server `<select>` (populated from `.AltTitleServers` server-side) + fetch button replacing current always-MangaDex button; subtle empty note when no providers
- [x] 5.2 Detail page: chips per alt title with `via <source>` badge + `×` remove affordance; click chip → POST action `set-title` with confirm (`onclick="return confirm(...)"` pattern consistent with existing destructive actions)
- [x] 5.3 Action endpoints mirroring the API handlers (303 redirect back to detail) — thin wrappers over the same bridge methods
- [x] 5.4 Library page: search box (`?q=` param on viewLibrary, server-side filter via `SearchLibrary`, results render in existing grid, stats row hidden when filtering)
- [x] 5.5 Tailwind regen if any new utility classes are introduced (regenerated via `npx tailwindcss@3.4.17 -c tailwind.config.js` from repo root + `make br`)

## 6. Verification

- [x] 6.1 Tests: DB layer (add/dedup/remove/swap/cascade), FTS sync (title swap reflected in search), fuzzy scoring order, bridge merge via fake provider
- [ ] 6.2 API tests in `api_test.go` style: all 5 endpoints happy path + validation errors
- [ ] 6.3 `make check` green
- [ ] 6.4 Live verify: restart server, fetch alt titles via UI dropdown, promote a title (library card + detail header change), remove a title, library search finds by alt title + typo case

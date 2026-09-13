## 1. Host text helpers

- [x] 1.1 Add `NormalizeStatus(map, raw)` and `DefaultStatusMap()` to `internal/pluginutil/status.go`; verify with a unit test covering known value, case-insensitive match, unknown passthrough, and nil-map default
- [x] 1.2 Register `host.text.normalize_status` in `internal/pluginmanager/lua_natives.go` as a callable table with a `default` field; verify a Lua fixture calls both `host.text.normalize_status(map, raw)` and `host.text.normalize_status(host.text.normalize_status.default, raw)`
- [x] 1.3 Register `host.text.normalize_status` in `internal/pluginmanager/js_natives.go` as a function carrying a `default` property; verify a JS fixture produces the same results as the Lua fixture
- [x] 1.4 Add a boundary test asserting Lua and JS return identical output for the same map/raw pairs through their VMs

## 2. Enrichment registry and built-in providers

- [x] 2.1 Create `internal/enrich` with `Kind`, `Item`, and the `Provider` interface; verify the package builds
- [x] 2.2 Implement `Registry` (`Register`, `Catalog(kind)`, `Resolve(kind, source)`) with built-in precedence over same-id plugin providers; verify with unit tests for catalog filtering, unknown-kind empty result, and collision precedence
- [x] 2.3 Implement `MangaDexProvider` (titles, categories, related) over `internal/hostnet`; verify with httptest fixtures for each kind and an error case
- [x] 2.4 Implement `MangaUpdatesProvider` (titles, summaries, categories, related) over `internal/hostnet`; verify with httptest fixtures for each kind and an error case
- [x] 2.5 Confirm the live MangaUpdates recommendation field and MangaDex `relationships` shape; verify by recording the API response into the test fixture and adjusting the parser until the fixture test passes

## 3. Persistence

- [x] 3.1 Add `manga_categories` and `manga_related` tables to the schema (`CREATE TABLE IF NOT EXISTS`, UNIQUE per manga+value); verify existing DB opens and migrates cleanly
- [x] 3.2 Add `AddCategories`/`ListCategories` and `AddRelated`/`ListRelated` DB methods using `INSERT OR IGNORE`; verify unit tests cover dedup and empty-input no-op
- [x] 3.3 Verify stored categories and related rows survive a re-fetch of overlapping items (no duplicates, existing rows intact)
- [x] 3.4 Add `GetEnrichment` to DB layer to retrieve all enrichment kinds for a manga row; verify unit test covers empty and populated results

## 4. Bridge and API

- [x] 4.1 Add bridge methods that resolve a provider by (kind, source), fetch, persist to the kind's table, and return the stored list; verify with bridge tests for a built-in source and an unknown source error
- [x] 4.2 Add `POST /api/manga/{pluginID}/{mangaID}/enrich` accepting `{kind, source}` and persisting/returning results; verify via handler test for success and unknown source/kind
- [x] 4.3 Route the existing `/alt-titles` and `/alt-summaries` endpoints through the registry with kind `titles`/`summaries`; verify existing handlers still pass their tests and a built-in source works with no plugin installed
- [x] 4.4 Include stored categories and related items in the manga detail view data; verify the detail view response contains them when rows exist
- [x] 4.5 Add `GET /action/fetch-enrichment/{pluginID}/{mangaID}` action handler for SPA form submit

## 5. Plugin provider adapter

- [x] 5.1 Parse `enrichment_providers` metadata (`{id, name, kinds[]}`) from the plugin Init response; verify a fixture declaring a custom source appears in the catalog
- [x] 5.2 Implement the plugin-backed `Provider` that calls the `GetEnrichment` export with `{title, kind, source}` and maps `{source, kind, items[]}`; verify a fixture provider round-trips items for a kind
- [x] 5.3 Verify a malformed or erroring `GetEnrichment` response yields a per-source error while built-in sources remain usable

## 6. Detail page UI

- [x] 6.1 Add the collapsed `<details>` enrichment panel (kind select, source select, GO) to `detail.lua`, with source options carrying `data-kinds`; verify it renders collapsed with no JS
- [x] 6.2 Add JS that filters source options to the selected kind and submits `{kind, source}` to the enrich endpoint, re-rendering the affected section; verify in the browser that selecting a kind narrows sources and GO populates the section
- [x] 6.3 Render stored categories and related items as detail sections; verify both appear after a fetch (covers loaded through the `/image` proxy)

## 7. Plugin migration and cleanup

- [x] 7.1 Migrate plugin `normalizeStatus` functions to `host.text.normalize_status` with each plugin's site-specific override map; verify each plugin's known raw values normalize to the canonical vocabulary
- [x] 7.2 Remove the built-in MangaDex/MangaUpdates bodies and their `alt_title_servers` declarations from shipped `enrich.lua`/`enrich.js`; verify no shipped plugin still declares the built-in sources and plugin loading is unchanged

## 8. Verification

- [x] 8.1 Run `make check` (fmt, race tests, lint) and confirm it is green
- [x] 8.2 Live smoke test: with no relevant plugin custom provider, fetch categories and related for a real library manga from MangaDex and MangaUpdates via the detail panel; confirm both sections populate and persist across a page reload

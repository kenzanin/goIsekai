# Move enrichment sources out of the host

## Why

Enrichment sources were host code: MangaDex and MangaUpdates provider implementations shipped inside `internal/enrich`, so adding a metadata source, fixing one when the upstream site changed, or trying a new one meant editing Go and rebuilding the host.

At the same time a second, older metadata path — plugins declaring `alt_title_servers` and exporting `getAltTitles`/`getAltSummary` — duplicated the same job through a different ABI, with its own HTTP routes, bridge methods, and template data. Its UI entry point was gone, so nothing could reach it, yet it stayed wired end to end and could drift from the path that actually ran.

Enrichment scripts replace the first problem with data (drop a folder, no rebuild) and make the second path redundant: with the surplus path deleted, there is one way to fetch metadata and one place to change it.

## What Changes

- **BREAKING**: the host ships no built-in enrichment providers. The MangaDex and MangaUpdates Go providers are deleted; `internal/enrich` keeps only the provider registry, the kind set, and the fetch/catalog logic over whatever providers are registered.
- Enrichment sources are discovered from Lua **info scripts** in the configured info directory (default `<data_dir>/info`, one folder per source, entry `main.lua`, folder name is the source id). Each script declares its providers in `PLUGIN.enrichment_providers` and implements `getEnrichment`. Adding or fixing a source is editing a script, not rebuilding the host.
- Info scripts are namespaced `info:<id>` in the plugin manager and excluded from the plugin listing, so an enrichment source never appears as a manga source, is never searched, and never serves chapter pages.
- Source ABI verification is skipped for info scripts. They are required to implement only `getEnrichment`; the `search`/`getMangaDetail`/`getChapterList`/`getPageList` globals a manga source must define are not checked for them.
- The enrichment catalog is assembled from info-script providers. Reading the catalog instantiates unloaded info scripts so their declared providers are visible, and leaves scraper plugins alone so the catalog does not pay for every source VM.
- **BREAKING**: `authors` joins the enrichment kinds. The fetched author is stored on the manga and used as the detail view's author when the source plugin supplies none.
- **BREAKING**: the plugin alt-title lookup API is removed end to end — `GET /api/alt-title-servers`, `POST /api/manga/{pluginID}/{mangaID}/alt-titles`, `POST /action/fetch-alt-titles/{pluginID}/{mangaID}`, `POST /action/fetch-alt-summaries/{pluginID}/{mangaID}`, the `alt_title_servers` plugin metadata field, the `getAltTitles`/`getAltSummary` plugin exports, and the `AppService.AltTitleServers`/`FetchAltTitles`/`FetchAltSummaries` bridge methods.
- Unchanged and still supported: alt-title storage and its detail-page controls, `PUT /api/manga/{pluginID}/{mangaID}/title` (promote an alternative to main title), `DELETE /api/manga/{pluginID}/{mangaID}/alt-titles` (remove one), the alt-summary equivalents, and `POST /action/fetch-enrichment/{pluginID}/{mangaID}` as the way to fetch metadata for a manga.

## Capabilities

### New Capabilities

None. Enrichment and alt-title handling both stay within existing capabilities.

### Modified Capabilities

- `enrichment`: the built-in MangaDex/MangaUpdates provider requirement is removed and replaced by discovery of enrichment scripts in the info directory; the catalog requirement no longer claims built-in entries; author enrichment is specified as a kind alongside titles, summaries, categories, and related.
- `plugin-runtime`: discovery now also scans the info directory, info scripts load without the manga-source ABI being verified, and they are excluded from the plugin list.
- `alt-titles`: both requirements describe the removed server-discovery and server-fetch API, so both are removed. What the capability used to cover now lives in `storage` (the alternative-titles table and dedup) and `enrichment` (fetching metadata from a source).
- `http-api`: the `GET /api/alt-title-servers` requirement is removed, and the alt-title management requirement keeps its list/delete behavior while losing the fetch half.

## Impact

- **Plugin ABI** (`pkg/types`): `PluginMeta.AltTitleServers`, `AltTitleServer`, `AltTitlesResult`, and `AltSummaryResult` are removed. `GetAltTitlesFunc` and `GetAltSummaryFunc` are dropped from the Lua, JS, and Yaegi ABI tables. Existing plugins that declared `alt_title_servers` or exported `getAltTitles`/`getAltSummary` must drop those, then declare `PLUGIN.enrichment_providers` and implement `getEnrichment` (or install an info script for the source they used).
- **Host packages**: `internal/enrich` (registry only), `internal/pluginmanager` (info-script discovery, `SetInfoDir`, `LoadEnrichmentProviders`, `ensureInfoLoaded`), `internal/bridge` (enrichment fetch, stored author), `internal/httpserver` (enrichment routes and detail view data), `internal/database` (manga author column, migration 21), `internal/config` (`info_dir`, defaulting to `<data_dir>/info`).
- **HTTP surface**: four routes removed; enrichment fetch, title promotion, and alt-title/alt-summary removal unchanged.
- **Templates and frontend**: the detail view's alt section keeps its promote/remove controls; the JS auto-expand helper that only served the removed fetch actions is gone.
- **Docs and examples**: `README.md` and `AGENTS.md` describe info scripts instead of built-in providers; `examples/info/mangadex/main.lua` is the reference script, and the per-plugin `enrich.js`/`enrich.lua` placeholders and `alt_title_servers` blocks are deleted.

# Add Enrichment Providers

## Why

Plugins each copy ~160 lines of alt-title/summary enrichment logic (10 byte-identical copies across JS and Lua plugins) that hit MangaDex and MangaUpdates directly, and status normalization is reimplemented 8 times with divergent results. Enrichment cannot currently fetch categories or related/recommended manga at all, and the detail page exposes one fetch button per provider. Moving provider logic to the host removes the duplication, adds new metadata kinds, and gives users one compact fetch control.

## What Changes

- Add a host-side enrichment registry with built-in Go providers for MangaDex and MangaUpdates covering four kinds: alternative titles, alternative summaries, categories (genres/tags), and related/recommended manga.
- Allow plugins to register custom enrichment providers for additional sources (e.g. a future `xxxx.net`), so a new source is a plugin addition, not a host rebuild.
- Persist fetched categories and related/recommended manga and render both on the manga detail page.
- Replace the per-provider fetch buttons with a single collapsed panel: `[what to fetch] [source] [GO]`.
- Add a `normalize_status` host text native taking `(default_map, raw)`; the host supplies a default map that plugins may override locally for site-specific upstream values.
- Migrate plugin inline `normalizeStatus` functions to the host native.
- Reduce plugin-side `enrich.lua` / `enrich.js` to thin dispatch, since MangaDex and MangaUpdates providers now live in Go.

## Capabilities

### New Capabilities

- `enrichment`: Host-side enrichment provider registry (built-in MangaDex/MangaUpdates providers plus plugin-declared custom providers), the four enrichment kinds (alt titles, alt summaries, categories, related manga), persistence, the unified fetch endpoint, and the compact detail-page fetch panel.
- `host-text-helpers`: Host text utility natives exposed to Lua and JS plugins; first member `normalize_status(default_map, raw)` with a plugin-overridable default map.

### Modified Capabilities

- `alt-titles`: Provider resolution now routes through the enrichment registry; MangaDex and MangaUpdates are host-native providers rather than plugin-delegated ones, and the requirement that the host hardcode no provider or server is superseded.
- `plugin-abi`: Adds the plugin-side enrichment provider contract (`GetEnrichment` export and provider metadata) used to register custom providers.

## Impact

- New code: `internal/pluginutil/enrich*.go`, `internal/pluginutil/status.go`; `internal/database/categories.go` and `related.go` (+ schema migration); bridge fetch methods; `internal/httpserver` enrich endpoints; `detail.lua` panel and new partials.
- Modified code: `internal/pluginmanager/{lua,js}_natives.go` (natives + registry wiring), `examples/plugins/**/enrich.{lua,js}` (thinned), plugin `main` files (normalizeStatus migration).
- Data: two new tables (manga categories, related manga) with UNIQUE dedup, following the existing `alt_titles`/`alt_descriptions` pattern.
- No new external dependencies; provider HTTP reuses the existing `internal/hostnet` tls-client stack.
- Affected existing specs: `alt-titles`, `plugin-abi`.

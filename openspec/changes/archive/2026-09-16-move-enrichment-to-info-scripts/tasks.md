# Tasks

## 1. Enrichment registry without built-in providers

- [x] 1.1 Reduce `internal/enrich` to the provider registry, the kind set, and the catalog/fetch logic over registered providers; delete the MangaDex and MangaUpdates provider files and their fixtures — verify: `CGO_ENABLED=0 go build ./internal/...` succeeds and no provider implementation remains under `internal/enrich`.
- [x] 1.2 Add `authors` to the enrichment kinds — verify: `CGO_ENABLED=0 go test ./internal/enrich/ -count=1` passes and `enrich.KindAuthors` resolves from `internal/bridge`.
- [x] 1.3 Add `info_dir` to the configuration, defaulting to `<data_dir>/info`, with parser and writer support — verify: a generated `goisekai.ini` contains `info_dir = <data_dir>/info` and the host starts when that directory does not exist.

## 2. Info-script discovery and loading

- [x] 2.1 Scan the info directory for `*/main.lua` during discovery, registering each folder as a Lua script marked info-only with an `info:<folder>` id — verify: `CGO_ENABLED=0 go test ./internal/pluginmanager/ -run TestInfoScript -count=1` passes.
- [x] 2.2 Point the manager at the info directory before discovery runs — verify: a startup with `app_data/info/mangadex/main.lua` present logs the script as registered.
- [x] 2.3 Keep info scripts out of the plugin listing — verify: `TestInfoScriptIsNotAMangaSource` passes and the Plugins screen shows no entry for the script.
- [x] 2.4 Skip manga-source ABI verification for info scripts, so a script defining only `getEnrichment` loads — verify: `TestInfoScriptLoadsWithoutSourceABI` passes while the same file placed in the plugin directory fails to load.
- [x] 2.5 Instantiate unloaded info scripts when the enrichment catalog is read, leaving manga source plugins unloaded — verify: `TestInfoScriptRegistersEnrichmentProvider` passes and a catalog read does not instantiate a registered-but-uninvoked source plugin.
- [x] 2.6 Isolate a script that fails to load, so the catalog still returns the remaining sources — verify: `TestInfoScriptUnknownKindIsEmpty` passes and a broken script produces a warning without an error response.

## 3. Enrichment scripts as sources

- [x] 3.1 Declare providers from the script's `PLUGIN.enrichment_providers` and route a fetch to the script's `getEnrichment` export — verify: `TestInfoScriptGetEnrichment` passes and a live fetch returns items tagged with the declared source.
- [x] 3.2 Ship `examples/info/mangadex/main.lua` as the reference script, memoizing the search-then-detail response across kinds — verify: `TestShippedMangaDexInfoScriptDeclaresKinds` passes and a live fetch against it stores titles, summaries, categories, authors, and related items.

## 4. Author enrichment

- [x] 4.1 Add the manga `author` column and its accessors — verify: the migration applies on an existing database and `CGO_ENABLED=0 go test ./internal/database/ -count=1` passes.
- [x] 4.2 Store the author captured during a fetch on the manga, joining multiple names and ignoring blanks — verify: a live fetch logs `enrich author stored` and the stored value holds the joined names.
- [x] 4.3 Fall back to the stored author in the detail view when the source plugin reports none, letting the plugin's author win when it does — verify: a live detail view for a manga whose plugin reports no author shows the stored value.

## 5. Remove the plugin alt-title lookup API

- [x] 5.1 Delete the ABI surface: the alt-title server and result types, the `alt_title_servers` metadata field, and the alt-title and alt-summary export constants — verify: `CGO_ENABLED=0 go build ./internal/... ./cmd/... ./pkg/...` succeeds and no Go file references `alt_title_servers` or the removed constants.
- [x] 5.2 Remove the Lua, JS, and Yaegi ABI entries and the manager's alt-title server listing and lookups — verify: `CGO_ENABLED=0 go test ./internal/pluginmanager/ -count=1` passes.
- [x] 5.3 Remove the bridge methods that listed servers and fetched titles or summaries, deleting their test and its fixture with them — verify: `CGO_ENABLED=0 go vet ./internal/... ./cmd/... ./pkg/...` is clean and `internal/bridge` has no reference to the removed methods.
- [x] 5.4 Remove the four routes and their handlers: the alt-title server list, the alt-title fetch, and the alt-title and alt-summary fetch actions — verify: each returns `404` against a running server while alt-title removal and main-title promotion still respond `200`.
- [x] 5.5 Drop the alt-title server data keys from the detail view and its templates, and remove the JavaScript auto-expand helper whose every call site was alt-title-specific — verify: no template or frontend file references the removed keys or helper, and the detail page still renders its promote and remove controls.

## 6. Docs, examples, and fixtures

- [x] 6.1 Describe the info-script model in `README.md` and `AGENTS.md`, replacing the built-in provider claims — verify: both document the info directory and neither claims a built-in MangaDex or MangaUpdates provider.
- [x] 6.2 Delete the per-plugin enrichment placeholders, the `alt_title_servers` blocks, and the JS alt-title test fixture — verify: no example plugin or installed plugin file references the removed metadata or enrichment requires.
- [x] 6.3 Replace the enrichment guidance in the Lua plugin example README with the info-script equivalent — verify: the example no longer advertises alt-title providers and points at the info directory instead.
- [x] 6.4 Repair the baseline specs this change is written against: the `enrichment` and `alt-titles` main specs carried their requirements under a leaked `## ADDED Requirements` and `## MODIFIED Requirements` header, so the parser saw no requirements and this change could not apply. Normalize both to `## Requirements` and refresh the `enrichment` Purpose, which the delta cannot carry for an existing capability — verify: `openspec validate move-enrichment-to-info-scripts --strict` passes and `openspec show move-enrichment-to-info-scripts --json --deltas-only` parses all 15 deltas.

## 7. Verification

- [x] 7.1 Full build and test suite — verify: `make build` and `CGO_ENABLED=0 go test ./internal/... ./pkg/... ./cmd/... -count=1` both pass.
- [x] 7.2 Lint and format gate — verify: `make check` reports zero issues.
- [x] 7.3 End-to-end live check — verify: the host starts with no error or warning log lines, an enrichment fetch for a real manga returns `200` and logs one `enrich fetch ok` per kind plus `enrich author stored`, the detail view shows the stored titles and author, and the four removed routes return `404`.

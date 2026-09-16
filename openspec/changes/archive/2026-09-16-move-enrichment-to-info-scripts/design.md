# Move enrichment sources out of the host

## Context

See `proposal.md` — Why. Three constraints shaped the approach:

- `internal/enrich` was a registry plus two hand-written providers (`mangadex.go`, `mangaupdates.go`). Callers already reached providers only by string id through the registry, so the host's fetch path was already source-agnostic; the two providers were the only host-specific parts.
- The plugin manager already models a script as a `loadedPlugin` with a lazy Lua VM, a per-plugin mutex, and a 15-second invocation timeout. An enrichment script is that same shape minus the manga-source ABI, so discovery and loading needed extending, not replacing.
- The alt-title lookup path was a second implementation of the same job: ABI constants and a metadata field in `pkg/types`, lookups in `pluginmanager`, fetchers in `bridge`, two API routes, two form actions, and template data keys. Its UI entry point had already been removed, leaving the whole path unreachable from the browser.

## Goals / Non-Goals

**Goals:**

- Adding, fixing, or removing a metadata source requires editing a script, never rebuilding the host.
- One way to fetch enrichment and one place to change it.
- An enrichment script can never be mistaken for, or reached as, a manga source.

**Non-Goals:**

- No change to how manga source plugins work: search, detail, chapter list, and page list are untouched.
- No change to alt-title storage, promotion to main title, or removal, and no change to their detail-page controls.
- No hot reload of info scripts. Discovery runs at startup, so a new or edited script needs a restart. A rescan can be added later without changing the requirements in `specs/`.
- No new sandboxing. An enrichment script gets the same Lua subset and the same `host.*` helpers as any Lua plugin, including the shared HTTP path through `internal/hostnet`.

## Decisions

**1. An info directory of scripts, not a plugin metadata field.** Sources are discovered from `*/main.lua` folders under `info_dir` (default `<data_dir>/info`), one folder per source, folder name = source id. The alternative — keep sources as plugins that declare `enrichment_providers` — was rejected because it conflates two different things: a manga source is installed to read chapters, a metadata source is installed to enrich. Tying one to the other means uninstalling a reader to drop a metadata source. A folder per source also makes "one file to edit and diff" true.

**2. Namespace ids as `info:<folder>` and hide them from `LoadedPlugins()`.** The same folder name legitimately appears in both directories (`mangadex` is both a plugin and a source), so the manager's map key is prefixed while the enrichment source id stays the bare folder name. `LoadedPlugins()` skips info scripts outright, which keeps them out of the plugin screen, out of search, and out of load-state reporting at the single point where the plugin list is produced. The alternative — a second map for info scripts — would have duplicated the lazy-load, mutex, and timeout machinery for no gain.

**3. Skip source-ABI verification for info scripts.** `loadLua` verifies that every manga-source global exists. An info script has none of them, so the check is skipped for namespaced ids while the `getEnrichment` check is skipped for everything (it is optional for a source plugin and meaningless without a declaration). The alternative — a per-kind ABI table — was rejected as new machinery serving a single case.

**4. Keep the provider interface, delete only the providers.** `enrich.Provider`, `Registry`, `Catalog`, `Resolve`, and `Fetch` are unchanged. Nothing in `bridge` or `httpserver` names a concrete provider, which is what made this a pure deletion rather than a migration: drop two files, two test files and their fixtures, and add the plugin-backed provider.

**5. Author is a field on the manga, not a row list.** A manga has one author, so `authors` items are joined into a single value stored on `mangas.author`, with blank names ignored. The alternative — reuse the alternative-titles table — would force the detail view to pick a winner at render time and would show the author in the promote/remove UI that exists for titles. The detail view keeps the source plugin's author when it supplies one and falls back to the stored value only when it does not.

**6. Delete the alt-title lookup path rather than leave it inert.** It could not be reached from the UI, and keeping it would leave a second implementation with its own ABI, routes, and tests, documented as supported. Deleting it makes the remaining surface honest: promotion and removal stay, fetching moves to the enrichment endpoint.

**7. Retire the `alt-titles` capability.** Both of its requirements describe the deleted servers/fetch API, so both are removed and the capability goes with them. The behavior that survives is already specified: the table and dedup in `storage`, fetching in `enrichment`, the endpoints in `http-api`. Writing replacement requirements to keep the file alive would specify behavior that already has a home.

## Risks / Trade-offs

- **Installed plugins that fetched metadata stop working** until they declare `enrichment_providers` and implement `getEnrichment` → both removed requirements carry a migration note, `README.md` and `AGENTS.md` describe the new model, and `examples/info/mangadex/main.lua` is a complete reference. The per-plugin `enrich.js` files deleted here were four-line placeholders, not working providers.
- **An empty catalog on upgrade** — `info_dir` defaults to `<data_dir>/info`, which does not exist yet → startup does not fail on a missing directory, and the docs point at the reference script to copy.
- **Retiring `alt-titles` deletes its main spec at sync time**, since removing both requirements empties the file → intended, and the surviving behavior is covered by the `storage`, `enrichment`, and `http-api` deltas. The spec was already non-conforming (a leaked delta with no Purpose section), so removing it also clears a `validate --all` failure.
- **Two baseline specs had no requirements section the parser recognises** → `openspec/specs/enrichment/spec.md` and `openspec/specs/alt-titles/spec.md` carried their requirements under a `## ADDED Requirements` and a `## MODIFIED Requirements` header, left behind by earlier archives. The parser reads only `## Requirements`, so it saw both specs as having no requirements: a `MODIFIED` delta would have aborted the archive with "not found", and a `REMOVED` delta would have silently warned and dropped nothing, leaving both files claiming the deleted API. Both headers were normalized to `## Requirements` in the same commit as this change, and the `enrichment` Purpose paragraph was rewritten (the delta cannot carry a Purpose for an existing capability). Without that, this change is not applicable to its own baseline — see `tasks.md` 6.4.
- **Reading the catalog instantiates info scripts** → the cost is one Lua VM per unloaded script and no network call, bounded by the number of folders under `info_dir`. Manga source plugins are explicitly left unloaded, so a large plugin directory costs nothing.
- **A script that fails to load silently drops its sources** → the failure is logged as a warning, the catalog returns the remaining sources, and the enrichment fetch reports a per-source error rather than failing the whole request.
- **A broken script breaks only its own source** → the provider boundary already turns a plugin call error into a fetch error for that source; `FetchAll` skips failing sources and continues.

## Migration Plan

1. Deploy. The only schema change is the additive manga `author` column (migration 21), applied automatically at startup.
2. For each source to keep, create `<data_dir>/info/<source>/main.lua` declaring `PLUGIN.enrichment_providers` and implementing `getEnrichment`. Copy `examples/info/mangadex/main.lua` as the template.
3. Restart, then confirm the source appears in the enrichment catalog and that a fetch stores items: `POST /action/fetch-enrichment/{pluginID}/{mangaID}` with a title and source, checking for `enrich fetch ok` and `enrich author stored` in the log.
4. In installed plugins, remove `alt_title_servers`, `getAltTitles`, `getAltSummary`, and any `enrich.js`/`enrich.lua` require. Clients reading `GET /api/alt-title-servers` must switch to the catalog.
5. Rollback is `git revert` of the two commits. The `author` column stays behind harmlessly; scripts in the info directory are ignored by the older code, and plugins that had already dropped `alt_title_servers` would need it restored to fetch alt titles through the old API.

## Open Questions

- Should info scripts be reloadable without restarting the host? Deferred — a rescan endpoint can be added later without changing any requirement in this change.
- Should the catalog report a script that failed to load, instead of only logging it? Deferred — the current behavior returns the remaining sources, which is a superset of what the requirements demand.

## Context

See `proposal.md` — Why. Current state that shapes the approach:

- Alt-title lookup is plugin-delegated: plugins declare `alt_title_servers` in metadata and export `GetAltTitles`/`GetAltSummary`; the host aggregates declared servers and calls the plugin. The shipped `enrich.lua` (byte-identical in 6 Lua plugins) and `enrich.js` (near-identical in 4 JS plugins) implement MangaDex and MangaUpdates HTTP + parsing inside the plugin sandbox.
- `pluginmanager` already registers host natives for plugins (`host.text.*`, `host.http.*`, `host.crypto.*`, `host.codecs.*`) in both runtimes via `lua_natives.go` / `js_natives.go`; Lua uses `mmcdole/lunar`, JS uses goja.
- Host HTTP goes through `internal/hostnet` (tls-client with TLS profiles, cookies, pacing) — not the sandbox `http_request` used by plugin code.
- Persistence follows a per-manga child-table pattern (`alt_titles`, `alt_descriptions`) with `INSERT OR IGNORE` dedup and a source label per row.
- The detail view is a Lua template (`internal/templates/views/detail.lua` + partials) rendered server-side; small pieces of vanilla JS handle interactivity (no Alpine in views).
- `normalizeStatus` is reimplemented in 8 plugin files with divergent keyword logic.

## Goals / Non-Goals

**Goals:**
- A single enrichment registry the host owns, holding built-in Go providers and plugin-declared providers behind one contract.
- Four enrichment kinds (titles, summaries, categories, related) with persistence and detail rendering for the new two.
- A compact, collapsed-by-default fetch panel on the detail page.
- `normalize_status` as a host native with a plugin-overridable map, callable identically from Lua and JS.

**Non-Goals:**
- Library word cloud / genre aggregation (deferred; the category data model will make it possible later).
- Removing the legacy `getAltTitles`/`getAltSummary` ABI exports (kept for compatibility; plugins may keep using them, but the shipped MangaDex/MangaUpdates copies are removed).
- Automatic/background enrichment or scheduled refresh — fetching stays user-triggered.
- Cross-source fuzzy matching of related titles beyond exact-title dedup.

## Decisions

### 1. Registry in a new `internal/enrich` package

A `Registry` maps `(kind, source)` to a `Provider`. Provider interface:

```
type Provider interface {
    Source() string                  // stable id, e.g. "mangadex"
    Name() string                    // display label, e.g. "MangaDex"
    Kinds() []Kind                   // subset of titles|summaries|categories|related
    Fetch(ctx, title string, k Kind) ([]Item, error)
}
```

`Item` is intentionally generic — `{ Value string; URL string; CoverURL string; Title string }` — because the four kinds are all small text/graph records; per-kind typed items would add types without behavior.

**Why over alternatives:** putting providers in `bridge` ties HTTP to the service layer; `pluginutil` is pure helpers with no network import and should stay that way. `internal/enrich` can import `internal/hostnet` without a cycle and is independently testable.

### 2. Built-in providers are Go implementations over `hostnet`

`MangaDexProvider` and `MangaUpdatesProvider` implement `Provider` directly. They reuse the host's tls-client stack (profile ladder, pacing) instead of the sandbox `http_request`, so built-in enrichment inherits the host's anti-bot and cookie behavior.

- MangaDex: search `GET /manga?title=...&limit=5`, read `attributes.altTitles` (titles), `attributes.tags[].attributes.name` (categories), and `relationships[]` where `type == "manga"` (related).
- MangaUpdates: `POST /v1/series/search` then `GET /v1/series/{id}`; read `associated` (titles), `description` (summaries), `categories`/`genres` (categories), and the recommendations/series fields (related).

**Alternative considered:** keep providers in plugin `enrich.lua`/`enrich.js` and only add categories there. Rejected — that preserves 10 duplicated copies and can't support related semantics consistently.

### 3. Plugin providers via a `GetEnrichment` ABI export (not a stored callback)

Plugins declare `enrichment_providers` in metadata (`[{id, name, kinds[]}]`) and export `GetEnrichment(arg)` where `arg = {title, kind, source}` → `{source, kind, items[]}`. The host wraps the plugin as a `Provider` that dispatches through the existing per-plugin call path (mutex, timeout, kind dispatch in `api.go`).

**Why over `host.enrich.register(fn)`:** a registration callback requires the host to retain a live reference into a VM and re-enter it later, which collides with lunar's non-reentrant `state.Call` and complicates the 15s timeout/panic isolation. The export model already exists as `GetAltTitles` and is proven; it also means provider code is plain plugin code, not a stored closure.

**Backward compat:** legacy `alt_title_servers` + `GetAltTitles` remain honored; when resolving a source id that both a built-in and a legacy plugin declare, the built-in wins (plugins shipping the old MangaDex declaration should drop it during migration).

### 4. One generic fetch endpoint; legacy alt-titles delegates to it

Add `POST /api/manga/{pluginID}/{mangaID}/enrich` with body `{kind, source}`. The registry resolves the provider, fetches, persists to the kind's table, and returns the updated stored list. The existing `/alt-titles` and `/alt-summaries` endpoints stay and delegate to the registry with kind `titles`/`summaries`, keeping current UI/API consumers working.

Storage:
- `titles` → existing `alt_titles`
- `summaries` → existing `alt_descriptions`
- `categories` → new `manga_categories (manga_row_id, category, source)` UNIQUE(manga_row_id, category)
- `related` → new `manga_related (manga_row_id, title, url, cover_url, source)` UNIQUE(manga_row_id, title)

All dedup via `INSERT OR IGNORE`, following the existing `AddAltTitles` pattern. Empty result is a no-op (never deletes rows).

### 5. `normalize_status` as a callable table in Lua, function-with-property in JS

The required API shape `host.text.normalize_status(host.text.normalize_status.default, raw)` needs `.default` to be readable off the native in both runtimes. JS supports this naturally (functions are objects). Lua functions cannot carry fields, so in Lua the native is exposed as a **table with a `__call` metamethod** plus a `default` field holding the default map table. Both runtimes present the same textual API.

- `internal/pluginutil/status.go`: `DefaultStatusMap() map[string]string` and `NormalizeStatus(m map[string]string, raw string) string`; case-insensitive key match; canonical vocabulary `Ongoing|Completed|Hiatus|Dropped|Upcoming`; unmatched → raw passthrough; empty/nil map → default map.
- Registered under `host.text.normalize_status` in `lua_natives.go` and `js_natives.go`.

**Alternative considered:** expose `host.text.status_default` as a separate global for Lua. Rejected — violates the requested uniform shape and forces per-language plugin code.

### 6. Detail panel uses native `<details>` + minimal filtering JS

The panel is `<details><summary>Enrichment</summary>` (native collapse, no JS) containing a kind `<select>`, a source `<select>`, and a GO button. Source `<option>`s carry `data-kinds`; a few lines of inline JS hide options whose kinds don't include the selected kind. On submit, POST to the generic endpoint and re-render the affected section (reload the section via the existing fragment-fetch style; simplest correct behavior).

## Risks / Trade-offs

- **MangaDex/MangaUpdates API shape or auth changes** → provider unit tests use recorded/httptest fixtures; failures degrade to a per-fetch error, never break the page.
- **Rate limits / `429` (MangaUpdates) and User-Agent requirements (MangaDex)** → built-ins go through hostnet pacing and set a stable UA; on 429 the fetch returns an error and stored data is untouched.
- **Lua callable-table support in lunar** → verify `setmetatable`/`__call` works through the native registration; fallback is a separate `status_default` global (documented deviation) if lunar cannot expose the field.
- **Built-in vs legacy plugin source-id collision** → registry resolution prefers built-in; migration removes the legacy MangaDex/MangaUpdates declarations from shipped plugins.
- **Related cover images** → related items may point at external cover URLs; render through the existing `/image` proxy so Referer/hotlink rules apply as for covers.
- **normalizeStatus migration drift** → migrate plugin-by-plugin and keep each plugin's site-specific keys in its override map; verify by comparing old vs new output on the plugin's known raw values.
- **DB migration** → additive `CREATE TABLE IF NOT EXISTS` in the existing schema list; rollback is dropping the two tables, no existing data touched.

## Migration Plan

1. Land `internal/enrich` (registry, built-ins, tests) and `internal/pluginutil/status.go`.
2. Add `manga_categories` / `manga_related` schema + DB methods + bridge fetches.
3. Add the generic `/enrich` endpoint and route alt-titles/alt-summaries through the registry; register the plugin adapter from metadata/`GetEnrichment`.
4. Add `host.text.normalize_status` to both runtimes; migrate plugin `normalizeStatus` functions to override maps.
5. Update `detail.lua` with the compact panel + categories/related sections.
6. Remove the built-in MangaDex/MangaUpdates bodies from shipped `enrich.lua`/`enrich.js` (and the legacy `alt_title_servers` entries covering them), leaving plugin custom providers only.
7. Verify: `make check`, provider unit tests, boundary tests through both VMs, then a live smoke test (detail fetch of categories and related on a real manga).

Rollback: revert the change; new tables are additive and unused by the prior binary.

## Open Questions

- Exact MangaUpdates endpoint/field for recommendations (vs MangaDex `relationships`) — confirm during implementation via live API probe; does not change the registry contract or tasks.
- Whether fetched categories should later feed the deferred library word cloud — deferred; the `manga_categories` table is designed to support it without change.

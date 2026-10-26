---
description: "Enrichment (info) scripts run as Odin wasm"
id: info-scripts-as-odin-wasm
title: Enrichment (info) scripts run as Odin wasm — the blocker was the load path, not the helper surface
type: finding
tags: [goisekai, plugins, info, enrichment, odin, wasm, pluginmanager]
---

# Enrichment (info) scripts run as Odin wasm

Asked 2026-10-20: "can the info scripts be Odin wasm?" First answer was no,
citing four blockers. Three of them turned out to be false, and the fourth was a
one-line assumption in discovery.

## The helper surface was already complete

Every host function the two info scripts call was already in `hostSurface` and
already dispatched by the wasm host before any of this work started:

| helper | used by |
| --- | --- |
| `json.decode` / `json.encode` | both |
| `http.get` / `http.get_body` | mangadex |
| `http.post_body` | mangaupdates (its search is a POST) |
| `text.normalize_title` | both |
| `text.url_encode` | mangadex |
| `text.strip_markdown` / `text.strip_link_blocks` | both |
| `log.debug/info/warn/error` | both |

So "the info scripts need helpers wasm doesn't have" was wrong. Nothing was
added to `host_surface.go` for either port — the parity test
(`TestWasmDispatchesEveryHelperItAdvertises`) already covered them.

## The actual blocker: discovery assumed `main.lua`

`Discover()` built the info entry as `filepath.Join(dir, "main.lua")` and
hardcoded `kind: "lua"`. A folder holding `main.wasm` was therefore either
skipped or registered as a Lua plugin pointing at a file that is not Lua.

Now the info block globs `infoDir/*` directories and picks the entry by
existence, with the same precedence plugin discovery uses: `main.wasm` wins over
`main.lua`, and a folder with neither is skipped. `kind` is derived, not
assumed. The id is still the folder basename, namespaced with the existing
`info:` prefix, which is what tells the rest of the manager this is an
enrichment script.

**Trap:** `wasmPath` means different things per runtime. `loadLua`, `loadJS` and
`loadYaegi` expect the **folder** (they append the entry filename themselves);
`loadWasm` expects the **.wasm file**. Getting this backwards yields
`main.lua/main.lua`. This was regressed once during the port and is the reason
the loaders each get their own variable rather than one shared path.

## Required exports differ per script kind

An info script exports `GetEnrichment` and **not** the scraper ABI
(`Search`, `GetMangaDetail`, `GetChapterList`, `GetPageList`). `loadWasm` used to
require the scraper set unconditionally, so a wasm info script would be rejected
at load. It now selects the required set from the `info:` id prefix:

```go
requiredFns := wasmRequiredFns
if isInfoPlugin(id) {
    requiredFns = wasmInfoRequiredFns // {types.GetEnrichmentFunc}
}
```

`GetEnrichment` also had to join the optional-registration list so `m.call` can
resolve it.

The build scripts mirror this: an info script's `build.sh` asserts
`contract_version malloc free GetEnrichment` plus the `env.host_call` import,
where a scraper's asserts the four ABI exports.

## Both installed scripts ported, A/B byte-identical

Source lives in `examples/info/*_wasm` (the user's steer); the canonical Lua
originals stay in `examples/info/*/main.lua` untouched as the archive copy.
Only `app_data/info/` was swapped.

| script | wasm size | notes |
| --- | --- | --- |
| `mangaupdates_wasm` | 407 KB | search-then-detail, single-slot memo |
| `mangadex_wasm` | 417 KB | reference-expanded search, 3 requests/title |

A/B method: a throwaway test built two managers (one pointed at the Lua folder,
one at the wasm folder), called `GetEnrichment` for all six kinds on the same
title, and diffed the full item arrays — not just the counts. Both scripts came
back **byte-identical on all six kinds**:

```
mangadex      titles 11, summaries 1, categories 4, authors 3, related 22, covers 9
mangaupdates  titles 11, summaries 1, categories 7, authors 4, related  5, covers 1
```

Live proof after install — the host log distinguishes the runtimes:

```
msg="info script registered" id=mangadex     kind=wasm path=app_data/info/mangadex/main.wasm
msg="info script registered" id=mangaupdates kind=wasm path=app_data/info/mangaupdates/main.wasm
```

## Bug found in the Lua while A/B-ing

`examples/info/mangaupdates/main.lua` only runs `variantMatch(record.title)`,
which returns false for any title under 3 tokens, and never compares
`record.title` for **exact** equality — only `associated[]` gets that check
(line 135). So `"Solo Leveling"`, a title the API returns verbatim, produced
**0 items for every kind**. Confirmed: 2-token title → Lua 0 items; 3-token
title → both scripts return data. The wasm port adds the exact comparison and
returns data for both.

## Plugins page lists them separately

`LoadedPlugins()` deliberately keeps excluding info scripts: it feeds plugin
search and the verify flow, where an enrichment script must not appear as a
manga source. Instead a separate `Manager.InfoPlugins()` accessor serves a new
"Info scripts" section on `/view/plugins`, showing name, runtime kind, id, site
URL and `built_with` per script.

## Cheapest path for the next info script

1. Copy `examples/info/<x>_wasm` from an existing port.
2. Write `main.odin` exporting `GetEnrichment`; call `abi.*` helpers only.
3. `build.sh` asserts `GetEnrichment` (not the scraper set).
4. Drop `main.wasm` + `plugin.json` into `app_data/info/<x>/`, delete `main.lua`.
5. A/B with the two-manager harness before trusting it.

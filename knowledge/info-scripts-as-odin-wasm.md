---
type: finding
title: Enrichment (info) scripts run as Odin wasm — discovery load-path fix
description: "Enrichment (info) scripts run as Odin wasm — the blocker was the load path, not the helper surface."
tags: [goisekai, plugins, info, enrichment, odin, wasm, pluginmanager, finding]
generated: { by: agent/mcp, at: "2026-10-26T15:41:15Z" }
status: stable
---

# Enrichment (info) scripts run as Odin wasm

Asked 2026-10-20: can the info scripts be Odin wasm? First answer was no, citing four blockers. Three were false, the fourth was a one-line assumption in discovery.

## The helper surface was already complete

Every host function the two info scripts call was already in hostSurface and already dispatched by the wasm host before any work started: json.decode/encode, http.get/get_body/post_body, text.normalize_title, text.url_encode, text.strip_markdown/strip_link_blocks, log.debug/info/warn/error.

So info scripts need helpers wasm doesn't have was wrong. Nothing was added to host_surface.go. The parity test (TestWasmDispatchesEveryHelperItAdvertises) already covered them.

## The actual blocker: discovery assumed main.lua

Discover() built the info entry as filepath.Join(dir, "main.lua") and hardcoded kind: lua. A folder holding main.wasm was therefore either skipped or registered as a Lua plugin pointing at a file that is not Lua.

Now the info block globs infoDir/* directories and picks the entry by existence, with the same precedence plugin discovery uses: main.wasm wins over main.lua. kind is derived, not assumed. The id is still the folder basename, namespaced with the info: prefix.

Trap: wasmPath means different things per runtime. loadLua, loadJS and loadYaegi expect the folder (they append the entry filename themselves); loadWasm expects the .wasm file. Getting this backwards yields main.lua/main.lua.

## Required exports differ per script kind

An info script exports GetEnrichment and not the scraper ABI. loadWasm used to require the scraper set unconditionally, so a wasm info script would be rejected at load. It now selects the required set from the info: id prefix: requiredFns := wasmRequiredFns; if isInfoPlugin(id) { requiredFns = wasmInfoRequiredFns }. GetEnrichment also had to join the optional-registration list so m.call can resolve it.

## Both installed scripts ported, A/B byte-identical

Source in examples/info/*_wasm. A/B: two managers (Lua folder vs wasm folder), GetEnrichment for all six kinds, diffed full item arrays — byte-identical:

| script | wasm size | manga kinds (counts) |
|---|---|---|
| mangaupdates_wasm | 407 KB | titles 11, summaries 1, categories 7, authors 4, related 5, covers 1 |
| mangadex_wasm | 417 KB | titles 11, summaries 1, categories 4, authors 3, related 22, covers 9 |

## Bug found in the Lua while A/B-ing

examples/info/mangaupdates/main.lua only runs variantMatch(record.title), which returns false for any title under 3 tokens, and never compares record.title for exact equality — only associated[] gets that check (line 135). So Solo Leveling, a title the API returns verbatim, produced 0 items for every kind. The wasm port adds the exact comparison and returns data for both.

## Plugins page lists them separately

LoadedPlugins() keeps excluding info scripts; a separate Manager.InfoPlugins() accessor serves a new Info scripts section on /view/plugins, showing name, runtime kind, id, site URL and built_with per script.

## Cheapest path for the next info script

1. Copy examples/info/<x>_wasm from an existing port.
2. Write main.odin exporting GetEnrichment; call abi.* helpers only.
3. build.sh asserts GetEnrichment (not the scraper set).
4. Drop main.wasm + plugin.json into app_data/info/<x>/, delete main.lua.
5. A/B with the two-manager harness before trusting it.

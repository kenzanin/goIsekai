---
type: Concept
title: Plugin runtime export matrix (lua vs js vs yaegi vs go vs wasm)
description: "Per-runtime ABI export names, metadata source, contract-version enforcement, and host-helper surface across lua/js/yaegi/go/wasm."
tags: [goisekai, plugins, wasm, lua, abi, audit]
generated: { by: agent/mcp, at: "2026-10-26T15:57:39Z" }
status: stable
---

# Plugin runtime export matrix

Traced from one question: "kenapa wasm tidak export function Init?" - into a full comparison of the five runtimes.

## The original question

**Wasm has no Init export because the host never looks for one.** loadWasm does: _initialize (reactor CRT), contract_version (checked), Search/GetMangaDetail/GetChapterList/GetPageList (required), GetGenres (optional). No mod.ExportedFunction("Init") lookup.

Discovery.go:18: "Wasm has no Init/PLUGIN export, so plugin.json is the only place." So wasm metadata comes from readPluginJSONMeta, not from a call.

## Export matrix

| | lua | js | yaegi | go .so | wasm |
|---|---|---|---|---|---|
| ABI map | luaFnNames | jsFnNames | yaegiABIFns | goFnNames | wasmRequiredFns |
| Search | search_manga | searchManga | Search | Search | Search |
| GetMangaDetail | get_manga_detail | getMangaDetail | GetMangaDetail | GetMangaDetail | GetMangaDetail |
| GetChapterList | get_chapter_list | getChapterList | GetChapterList | GetChapterList | GetChapterList |
| GetPageList | get_page_list | getPageList | GetPageList | GetPageList | GetPageList |
| GetGenres | get_genres (opt) | getGenres (opt) | GetGenres (opt) | GetGenres (opt) | GetGenres (opt) |
| GetEnrichment | getEnrichment (opt) | | | | |
| Init | | | Init (opt) | | |

## Return convention

- lua: JSON string, or Lua table (auto-encoded), or nil, "code".
- js: string, or any value marshalled to JSON.
- yaegi/go: (string, error); host requires result to parse as JSON.
- wasm: JSON in/out over linear memory; packed i64 in/out.

## Metadata source

| Runtime | Meta from | Init? |
|---|---|---|
| lua | PLUGIN table | no |
| js | PLUGIN object | no |
| yaegi | Init() return (optional) | yes |
| go | Meta var | no |
| wasm | plugin.json | no |

## Contract-version enforcement

| Runtime | Declared where | Enforced? |
|---|---|---|
| lua | PLUGIN.contract_version | yes (CheckVersion) |
| js | hardcoded int32(1) | no (cannot fail) |
| yaegi | hardcoded 1 | no (never called) |
| go | ContractVersion var | yes |
| wasm | contract_version export | yes |

## Host-helper surface

- lua: ~40 flat globals + namespaced tables (text, codecs, crypto, json, html, regex, http, browser).
- wasm: exactly two imports - env.host_http_request + env.host_call (25-entry dispatch: text.chapter_num, text.normalize_status, json.decode/encode, regex.{find,match,find_all,find_index,replace}, crypto.vrf_sign, html.{parse,find_text,find_attr,find_list_text,find_list_attr,xpath_*}, http.get/post/get_body/post_body).
- js: own natives (js_natives.go).

## Inconsistencies (resolved 2026-10-17)

1. Init doc corrected, not implemented - metadata via plugin.json for all runtimes.
2. GetMangaDetailWithChapters removed (dead optimization, no runtime maps it).
3. plugin.json is single metadata source for every runtime.
4. Enrichment split off source ABI (luaInfoFnNames vs luaFnNames).
5. Naming unified per language (Search is search in lua/js, was search_manga/searchManga).
6. Wasm helper-surface parity: still open.

## Files

internal/pluginmanager/: lua.go, lua_load.go, js.go, yaegi.go, goplug.go, wasm.go, wasm_abi.go, wasm_host_dispatch.go, api.go, api_detail.go, discovery.go, manager.go. pkg/types/abi.go, pkg/types/version.go.

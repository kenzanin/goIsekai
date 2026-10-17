---
type: Concept
title: Plugin runtime export matrix (lua vs js vs yaegi vs go vs wasm)
description: Traced 2026-10-17 from the question "kenapa wasm tidak export function Init?" - per-runtime ABI export names, meta source, contract-version enforcement, host-helper surface, and the inconsistencies worth normalizing.
tags:
  - goisekai
  - plugins
  - wasm
  - lua
  - abi
  - audit
generated:
  by: process:reasonix
  at: "2026-10-17T00:00:00Z"
---

# Plugin runtime export matrix

Traced from one question — *"kenapa wasm tidak export function `Init`?"* — into
a full comparison of the five runtimes. Verified by reading the loaders
(`loadLua`, `loadJS`, `loadYaegi`, `loadGo`, `loadWasm`) and their dispatch maps
via codegraph, not by assumption.

## The original question

**Wasm has no `Init` export because the host never looks for one.** `loadWasm`
(`internal/pluginmanager/wasm.go:67`) does exactly this, in order:

1. `_initialize` — the WASI **reactor CRT** entry point, run once so the module
   reaches a ready state. It is *not* the ABI `Init`; easy to confuse.
2. `contract_version` — required, checked with `types.CheckVersion`.
3. `Search`, `GetMangaDetail`, `GetChapterList`, `GetPageList` — required.
4. `GetGenres` — optional.

There is no `mod.ExportedFunction("Init")` lookup, and `git log -S'InitFunc'
-- internal/pluginmanager/wasm.go` is empty — the wasm loader never had that
path. The reason is written in `internal/pluginmanager/discovery.go:18`:

> Wasm has no Init/PLUGIN export, so **plugin.json is the only place** a wasm
> plugin can declare its site_url (used as the default image Referer).

So wasm metadata comes from `readPluginJSONMeta` (`wasm.go:132`), not from a
call. A C++ plugin exporting `Search`/`GetMangaDetail`/`GetChapterList`/
`GetPageList`/`GetGenres`/`contract_version` and no `Init` is correct.

## Export matrix

| | lua | js | yaegi | go (`.so`) | wasm |
|---|---|---|---|---|---|
| **ABI map** | `luaFnNames` (lua.go:19) | `jsFnNames` (js.go:19) | `yaegiABIFns` (yaegi.go:22) | `goFnNames` (goplug.go:29) | `wasmRequiredFns` (wasm.go:25) |
| **Naming style** | `snake_case` globals | `camelCase` globals | Go `func` in source | exported Go symbols | `export_name` symbols |
| **Search** | `search_manga` | `searchManga` | `Search` | `Search` | `Search` |
| **GetMangaDetail** | `get_manga_detail` | `getMangaDetail` | `GetMangaDetail` | `GetMangaDetail` | `GetMangaDetail` |
| **GetChapterList** | `get_chapter_list` | `getChapterList` | `GetChapterList` | `GetChapterList` | `GetChapterList` |
| **GetPageList** | `get_page_list` | `getPageList` | `GetPageList` | `GetPageList` | `GetPageList` |
| **GetGenres** | `get_genres` (opt) | `getGenres` (opt) | `GetGenres` (opt) | `GetGenres` (opt) | `GetGenres` (opt) |
| **GetEnrichment** | `getEnrichment` (opt) | ✗ not mapped | ✗ | ✗ | ✗ |
| **Init** | ✗ | ✗ | `Init` (opt) | ✗ | ✗ |
| **required set** | 4 (enrich+genres opt) | 4 (genres opt) | 4 | 4 (genres opt) | 4 (genres opt) |

### Return convention

- **lua** — returns a JSON string, **or a Lua table** (auto-encoded via
  `lunarTableToJSON`), or `nil, "<code>"` for failure. A bare bool is rejected.
- **js** — returns a string, or any value that is then marshalled to JSON.
- **yaegi / go** — return `(string, error)`; the host additionally requires the
  result to parse as JSON (`callGo`/`callYaegi` probe with `json.Unmarshal`).
- **wasm** — always JSON in / JSON out over linear memory; a `(ptr|len<<32)`
  packed i64 in and out, buffers from the guest's own `malloc`.

## Metadata source per runtime

| Runtime | Meta comes from | Init used? |
|---|---|---|
| lua | `PLUGIN` table (`lua_load.go:48`) | ✗ |
| js | `PLUGIN` object (`js.go:129`) | ✗ |
| yaegi | `Init()` return value, if the source defines it (`yaegi.go:152`) | ✅ optional |
| go | exported `Meta` variable of type `types.PluginMeta` | ✗ |
| wasm | `plugin.json` on disk (`readPluginJSONMeta`) | ✗ |

`Init` is **yaegi-only**, and `initWrapperSrc` (`yaegi_helpers.go:20`) generates
a wrapper because its signature varies — no arg or one arg, returning `string`
or `(string, error)`.

## Contract-version enforcement

| Runtime | Declared where | Actually enforced? |
|---|---|---|
| lua | `PLUGIN.contract_version` (table field) | ✅ `CheckVersion` (lua_load.go:79); missing/non-number is a load error |
| js | `const contractVer := int32(1)` in the loader | ⚠️ `CheckVersion` runs (js.go:141) but against a **hardcoded 1**, so it can never fail — `PLUGIN.contract_version` is silently ignored |
| yaegi | hardcoded `contractVersion: 1` (yaegi.go:148) | ❌ `CheckVersion` is never called |
| go | exported `ContractVersion` int32 var | ✅ `CheckVersion` (goplug.go:55) |
| wasm | exported `contract_version` function | ✅ `CheckVersion` (wasm.go:106); missing export is a load error |

## Host-helper surface (the widest gap)

- **lua** injects ~40 flat globals plus namespaced tables: `text`, `codecs`,
  `crypto`, `json`, `html`, `regex`, `http`, `browser`, plus `evaluate`,
  `evaluate_with_init`, `fetch`. Includes `base64_encode`, `base64url_*`,
  `hex_*`, `sha256_hex`, `md5_hex`, `hmac_sha256_hex`, `aes_gcm_decrypt`,
  `xor`, `substitute_cipher`, `date_to_iso`, `titlecase`, `trim`, `unescape`.
- **wasm** has exactly **two** imports: `env.host_http_request` and the generic
  `env.host_call`, whose dispatch table (`wasm_host_dispatch.go`) is 25 entries:
  `text.chapter_num`, `text.normalize_status`, `json.decode`, `json.encode`,
  `regex.{find,match,find_all,find_index,replace}`, `crypto.vrf_sign`,
  `html.{parse,find_text,find_attr,find_list_text,find_list_attr,xpath_text,
  xpath_attr,xpath_list_text,xpath_list_attr}`, and
  `http.{get,post,get_body,post_body}`.
- **js** has its own native set (`js_natives.go`), roughly mirroring lua's.

Missing on wasm relative to lua: `browser.evaluate`, base64/hex codecs, sha256/
md5/hmac, aes_gcm_decrypt, xor, substitute_cipher, date_to_iso, and the flat
text helpers beyond `chapter_num`/`normalize_status`.

## Inconsistencies worth normalizing

1. **`Init` is documented as if it were universal.** `pkg/types/abi.go:25` says
   *"an optional export. When present, the host calls it once at load time"*,
   but `types.InitFunc` is referenced only by `yaegi.go` /
   `yaegi_helpers.go`. For wasm the doc is actively wrong (no lookup exists);
   for lua/js/go it is wrong by omission. The accurate statement lives in the
   `discovery.go` comment. Either clarify the doc, or implement the optional
   lookup in the wasm loader (plugin.json as fallback) — decision not taken.
2. **`GetMangaDetailWithChapters` is a dead optimization path.** It is a real
   manager method (`api_detail.go:119`, with fallback to two separate calls) and
   `pkg/types/abi.go:19` documents it, but **no runtime maps it** — not in
   `luaFnNames`, `jsFnNames`, `goFnNames`, `yaegiABIFns` or `wasmRequiredFns`.
   `m.call` therefore always fails for it and the code always takes the
   fallback. No production caller either (codegraph: 0). Either wire it into the
   runtimes or drop it with `pkg/types/abi.go:19`.
3. **JS contract version is not real.** `CheckVersion(int32(1))` with a literal
   cannot reject anything; the value a JS plugin declares in `PLUGIN` is never
   read. Compare `contract_version` in lua, go and wasm, all of which enforce.
4. **Enrichment is lua-shaped.** Only `luaFnNames` maps `GetEnrichment`. JS has
   no mapping, and wasm/go/yaegi have none either, so enrichment scripts are
   effectively lua-only despite `GetEnrichmentFunc` being a documented optional
   ABI entry.
5. **Wasm helper surface is roughly half the lua one.** A plugin ported from lua
   to wasm usually has to reimplement codecs/crypto locally — which is exactly
   what `wasm-plugin-local-helpers-decision.md` decided to keep doing for hot
   paths. Worth knowing before promising a straight port.
6. **Naming has three conventions** (snake_case / camelCase / Go-exported) for
   the same five entry points, so any host-side refactor or doc must translate
   per runtime rather than name ABI entries directly.

## Files

- `internal/pluginmanager/`: `lua.go`, `lua_load.go`, `js.go`, `js_console.go`,
  `yaegi.go`, `yaegi_helpers.go`, `yaegi_call.go`, `goplug.go`, `wasm.go`,
  `wasm_abi.go`, `wasm_host_dispatch.go`, `api.go`, `api_detail.go`,
  `discovery.go`, `manager.go`
- `pkg/types/abi.go`, `pkg/types/version.go`

## Resolution (2026-10-17)

The normalisation candidates above were settled in one change set:

1. **`Init` doc corrected, not implemented.** `types.InitFunc` is now documented
   as a legacy export name: metadata lives in `plugin.json` for every runtime and
   no runtime reads an `Init` export anymore. The yaegi-only lookup and the
   `initWrapperSrc` wrapper (48 lines, one caller) were deleted, and the `Init`
   exports were dropped from the two example yaegi plugins.
2. **`GetMangaDetailWithChapters` removed.** The constant and the never-called
   `Manager` method are gone; the ABI is five entry points.
3. **`plugin.json` is the single metadata source for every runtime.** All five
   loaders stopped parsing meta out of the plugin (lua `PLUGIN` table, js `PLUGIN`
   object, yaegi `Init()` + JSON, go `Meta` var) and read `plugin.json` through
   the shared `pluginMetaFor`/`readPluginJSONMeta` helper instead — which is what
   wasm already did. Ten symbols plus `initWrapperSrc` went with it.
   `PluginMeta` gained `Description` and `ContractVersion`; a plugin whose
   `plugin.json` declares no version is treated as current, never as v0.
   Migration shipped `plugin.json` for 37 installed plugins, 51 examples and all
   info scripts; `TestEveryShippedPluginDeclaresMetadata` holds the invariant.
   This found a real gap: `manhawato` had only `site_url`, so it would have
   displayed nameless.
4. **Enrichment split off the source ABI.** `luaInfoFnNames` holds
   `get_enrichment`; `luaFnNames` is the source ABI only, so a source plugin is
   never verified against an enrichment entry point.
5. **Naming unified per language convention.** Search is `search` in Lua and JS
   (was `search_manga`/`searchManga`); `GetEnrichment` is `get_enrichment` (was
   the sole camelCase entry in the snake_case Lua map). Legacy names still
   resolve, with a deprecation warning, so 64 Lua + 6 JS files needed no
   migration.
6. Wasm helper-surface parity: still open, untouched.


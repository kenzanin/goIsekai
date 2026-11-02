---
type: guide
title: "Plugin authoring guide: what the host requires and what to return"
description: "Checklist for writing or fixing a goIsekai plugin: folder layout, plugin.json fields and their effects, required Search/GetMangaDetail/GetChapterList/GetPageList JSON contracts per runtime naming, optional GetGenres, host.* helper namespaces, and the wire-shape rules and incidents to reuse."
tags: [goisekai, plugins, abi, pluginjson, authoring, contract, guide, kind, host-helpers]
generated: { by: agent/mcp, at: "2026-11-02T15:24:08Z" }
status: stable
---

## Plugin = 3 files in a folder

`app_data/plugins/<plugin-id>/` (and source copy under `examples/plugins/<runtime>/<plugin-id>/` - keep both in sync via `cp`):
- `plugin.json` - metadata, read WITHOUT instantiating any runtime.
- entry file: `main.lua` | `main.js` | `main.go` | `main.wasm` (+ build tool when compiled).
- `logo.png` - optional; or set `logo` to an absolute URL or data URI.

## Two plugin types: source scraper vs enrichment (info)

The guide below (bottom section) applies to SOURCE plugins. Enrichment/info scripts are a DIFFERENT type with their own contract:
1. Folder lives in `app_data/info/<id>/` (not `app_data/plugins/`); manager ids get the `info:` prefix so an info script can coexist with a source plugin of the same folder name.
2. The ONLY export is `GetEnrichment` - input `{"title","kind","source"}`, output `[{"value","url","cover_url","source"}]`. It never scrapes a manga source and never shows up as a library source; the UI surfaces it as an enrichment source in the alt-detail picker.
3. `plugin.json` declares `enrichment_providers: [{"id","name","kinds":[...]}]`. Kinds today: titles, summaries, categories, authors, related, covers (internal/enrich/enrich.go). `precedence` = run order (lower runs first, default highest). `enabled` defaults true.
4. If both main.wasm and main.lua exist in an info folder, wasm wins (same precedence as source discovery: wasm scanned before yaegi because a built wasm folder carries main.go).
5. Contract_version still applies.

Reference built info plugins: app_data/info/mangaupdates, app_data/info/mangadex (odin wasm, built_with stamped).

## plugin.json (all fields optional, host falls back to defaults)
```json
{
  "name": "Mangzio",            // human display name (falls back to plugin id)
  "site_url": "https://www.mangzio.com",
  "verify_url": "https://www.mangzio.com",  // where user solves anti-bot challenges
  "logo": "logo.png",            // filename, absolute URL, or data URI
  "thumb_ratio": 0.703,          // cover w/h for UI space reservation; 0 -> 2:3
  "contract_version": 1,         // current ABI = types.ContractVersion (1)
  "paginates": true,             // plugin pages source itself; host then does NOT slice results. Absent/false = you return ALL matches, host slices by searchPageSize.
  "search_page_size": 24,        // declare page size when paginating (mangzio lesson)
  "timeout": 30,                 // override default 15s invoke deadline (many-RTT calls only)
  "needs_human_verify": false,   // site challenges need manual wizard
  "needs_js": false,             // render via browser engine instead of fast path
  "manual_cookies": false,       // anti-bot cookies pasted by user, never solved
  "http_profiles": ["chrome_146"], // TLS profile ladder (tls-client names or "stdlib")
  "built_with": "tinygo 0.43.0", // stamped by build script, informational
  "params": { "site_url": "...", "cdn": "..." }  // editable facts handed back via host.plugin.param(name) - put things users may edit here, NOT in code
}
```
Extras: `enrichment_providers` (declares custom enrichment sources: `{id,name,kinds[],precedence,enabled}`).

## Required exports (host accepts only correctly-shaped data from the plugin)
Each export takes a JSON string, returns a JSON string.
1. `Search(arg)` - arg = SearchFilter {"query","page","genres":[],"sort_by"}. Returns JSON array of {"id","title","cover_url", optional "genres":[".."], "author","description","status","rating"}. Use STABLE ids/slug matches; DO NOT deduplicate by array position.
2. `GetMangaDetail(arg)` - arg = string manga id. Returns {"id","title","cover_url","description","author","status","genres",..,"rating"}. Genres here also power the detail page tags AND library fallback.
3. `GetChapterList(arg)` - arg = manga id. Returns array of {"id" ("<slug>:chapter-N"), "manga_id", "title", "chapter_num" (float), "volume_num", "released_at", "url", "scanlator"}. ORDER: newest first in the plugin (host re-checks nothing).
4. `GetPageList(arg)` - arg = chapter id. Returns array of {"index","url","headers"{}}; put CDN Referer/User-Agent in per-page headers when the CDN gates on it.

Optional: `GetGenres()` - no args; returns [{"name","slug"}] powering the /view/search genre filter (bridge ListGenres cache). Naming per runtime: Lua snake_case (get_manga_detail), JS camelCase (getMangaDetail), WASM/native/yaegi PascalCase (GetMangaDetail). Contract version export: WASM func `contract_version`; native Go exports int32 var; Lua PLUGIN.contract_version; JS/yaegi assume the current ABI.

## Wire-shape rules that have bitten us
- `released_at`: host routes by JSON type. NUMBER = epoch SECONDS only (millis rejected by 1970-2100 range guard => "no date"). STRING = RFC3339 parsed, else kept verbatim as `released_text` display wording. Prefer `host.text.date_to_iso(...)` to normalize any source date into RFC3339 before returning; if the source has none, return nothing rather than "recently".
- Detail/chapter split: detail page = cover picks; chapter list = dates. Do NOT re-fetch detail on cover changes (leak lesson).

## Host helpers (same Go implementations behind JS `host.*`, Lua `host.*`, and wasm `host_call` {"fn":"ns.name","args":[..]})
- `host.http.get/get_body/post/post_body` (JS+Lua); wasm plugins import ONE `host_http_request`: HTTPRequest {method,url,headers,body} -> HTTPResponse {status,headers,body}.
- `host.text.*` - normalize_status, date_to_iso, strip_html, trim, url_encode, chapter_num, js_escape/lua_escape, unescape.
- `host.json.decode/encode`, `host.text.json_blob` (balanced-target walk: use this instead of slicing brackets from strings).
- `host.html.parse / find_text / find_attr / find_list_text / find_list_attr` (CSS selectors); `host.html.xpath_*`.
- `host.regex.match/find/find_all/find_index/quote/replace`.
- `host.browser.evaluate / evaluate_with_init / fetch` - for JS-rendered sites (pairs with needs_js).
- `host.crypto.vrf_sign`, substitute_cipher etc. (site-specific natives).

## Authoring rules from incidents
1. Never guess byte windows for related metadata; parse the balanced object (mangzio genres sat ~1.5KB past the card fields - an 800-byte window missed them).
2. After adding/removing ANY optional export (e.g. GetGenres), reset the host caches that recorded its absence: `UPDATE plugins SET genres = NULL WHERE id='..'` (ListGenres cache is sticky: "[]" forever skips).
3. node --check catches syntax, not ReferenceError; always verify LIVE API after each JS plugin edit (curl /api/search...) - a ReferenceError surfaced as HTTP 500 only live.
4. Cover dims, alt covers, enrichment: detail is source-of-truth for user-visible metadata ONLY; the cover picker does not re-run detail.
5. Restart gate: `pkill -x goisekai; sleep 2; (setsid nohup ./goisekai > goisekai.log 2>&1 &); sleep 8; curl -s -o /dev/null -w "%{http_code}" http://127.0.0.1:8080/` = 200.

## Reference plugins to copy from
- mangzio (JS, examples/plugins/js/mangzio/main.js): RSC chunk extraction + balanced object + getGenres.
- weebcentral (Lua): get_genres cached in a module-local var.
- dynastyscan (Lua): JSON-API based, taggings->chapters.
- bato1 (native Go): complete reader-page flow with per-page headers.
- mangakatana_odin / violetmanga_odin (wasm): chapter parsing tables, `host_call` dispatch, json_blob use.
- app_data/info/mangaupdates, app_data/info/mangadex (enrichment/odin wasm): GetEnrichment contract + enrichment_providers kinds list.

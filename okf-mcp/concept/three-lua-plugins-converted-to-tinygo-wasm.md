---
id: three-lua-plugins-converted-to-tinygo-wasm
title: atsumaru + lhtranslation + mangabuddy ported from Lua to TinyGo WASM
description: Three Lua plugins converted to TinyGo WASM (Typesense/Madara-HTML/JSON+HTML), covering hit["document"] nesting, bareSlug, decodeString sandbox fallback, the stale main.lua that made the manager load them as Lua, and the gAll helper mis-parsing regex.find_all's [][]string rows (silently returned 0 pages).
type: port
tags: [goisekai, plugins, wasm, tinygo, atsumaru, lhtranslation, mangabuddy, port, regex]
generated:
  at: "2026-10-26T00:00:00Z"
---

# Three Lua Plugins Converted to TinyGo WASM

## Summary

Converted three Lua plugins (`atsumaru`, `lhtranslation`, `mangabuddy`) to TinyGo WASM, mirroring the existing `1manga` plugin structure. All four wasm plugins build via Makefile (TinyGo 0.43.0, `wasm-opt -O4`) — never via `tools/build-wasm.sh`.

## Changes

### atsumaru (`examples/plugins/wasm/atsumaru/`) — Typesense JSON API
- **Files**: `main.go`, `helpers.go`, `text.go`, `text_host.go`, `hostcall.go`, `abi.go`, `go.mod`, `Makefile`, `plugin.json`, `logo.png`
- **Port**: search via Typesense (`/documents/search`), detail via `/api/manga/page`, chapters via `/api/manga/allChapters`, pages via `/api/read/chapter`
- **Search title fix**: Typesense API nests each hit's document under `hit["document"]`; `search()` reads `hit["document"]` instead of `hit` directly (otherwise empty titles).
- **decodeString** (`text.go`): handles both bare and JSON-encoded strings.

### lhtranslation (`examples/plugins/wasm/lhtranslation/`) — Madara HTML scraping
- **Files**: `main.go`, `helpers.go`, `text.go`, `text_host.go`, `hostcall.go`, `abi.go`, `go.mod`, `Makefile`, `plugin.json`, `logo.png`
- **Port**: search via `/`, detail via `/manga/{slug}`, chapters via POST `/ajax/chapters/`, pages via `/manga/{slug}/{chapter}`
- **decodeString** (`helpers.go`): falls back to raw arg when JSON unmarshal fails — sandbox passes bare string.
- **pages regex**: `data-src="([^"]*(?:\.jpg|\.jpeg|\.png|\.webp))"` + `strings.TrimSpace` on capture.

### mangabuddy (`examples/plugins/wasm/mangabuddy/`) — JSON+HTML hybrid
- **Files**: `main.go`, `helpers.go`, `text.go`, `text_host.go`, `hostcall.go`, `abi.go`, `go.mod`, `Makefile`, `plugin.json`, `logo.png`
- **Port**:
  - search via `/api/search?search=<q>` (JSON) → `slug_hash` as ID + `title` + `image`
  - detail via `/get-chapter-list?slug=<bareSlug>` (JSON) + `/api/search?search=<slug>` for title+cover
  - chapters via `/get-chapter-list?slug=<bareSlug>` (JSON) → `chapter_slug`/`chapter_name`/`chapter_num`/`updated_at`
  - pages via `/series/<mangaID>/<chSlug>` (HTML) → `data-src="https://{cdn}/{path}.webp"`
- **`bareSlug()`**: returns first `.`-delimited segment (strips `.ZDOePg` / `.zid` suffixes).
- **Safe type assertions** (comma-ok) throughout to prevent wasm panic on missing JSON fields.

#### mangabuddy `pages()` returned 0 images — `gAll` mis-parsed the host's regex response

`pages()` regex is correct (live chapter HTML has `data-src="https://cdn4.sags2mobile.com/<slug>/<ch>/<i>.webp"`, 17–56 imgs/chapter). The bug was the `gAll()` helper in `helpers.go`: it unmarshalled the host's `regex.find_all` response as `[]int` (ranges) with a `[][]int` fallback — but the host returns **`[][]string` rows of capture groups**, so both unmarshals failed, `gAll` returned `nil`, and `pages()` yielded 0. lhtranslation's known-good `gAll` (→ `firstCapAll`) parses `[]json.RawMessage` rows → `[]string` and takes `vals[0]`. Fix: mangabuddy `gAll` now uses the same row parsing.

- **Trap:** `regex.find_all` (host) ≠ `regex.find`. `find` returns one flat `[]int` of ranges (`[start,end,...]`); `find_all` returns `[][]string` (per-match capture groups). A helper written for one shape silently returns empty on the other — no error, just 0 results.
- Live after fix: chapter-10=17, chapter-5=46, chapter-1=56; second manga chapter-3=17.

## Build Process

All plugins share the same Makefile pattern (TinyGo 0.43.0):
```makefile
main.wasm: main.go hostcall.go text.go text_host.go abi.go
	$(TINYGO) build -target wasi -buildmode=c-shared -o main.wasm .
	$(WASMOPT) -O4 main.wasm -o main.wasm
```

## Host bridge note (no code change)

`internal/bridge/library.go` `GetMangaDetail` returns `cachedMangaFallback` when a plugin returns empty `Title` (line 82–84). Plugins must populate `Title`/`MangaID` to avoid the fallback — all three do.

## Live Verification

All four plugins installed to `app_data/plugins/{id}/main.wasm`, reloaded, cache cleared, server restarted.

| Plugin      | Search | Detail (title) | Chapters (manga_id) | Pages |
|-------------|--------|-----------------|----------------------|-------|
| 1manga      | ✅     | ✅              | ✅                   | ✅    |
| atsumaru    | ✅     | ✅              | ✅                   | ✅    |
| lhtranslation| ✅    | ✅              | ✅                   | ✅    |
| mangabuddy  | ✅     | ✅              | ✅                   | ✅    |

## OKF References

- `okf-mcp/concept/1manga-two-stacked-graphql-bugs.md` — 1manga bug analysis
- `okf-mcp/concept/lhtranslation-decodeString-sandbox-string-format.md` — decodeString sandbox format issue
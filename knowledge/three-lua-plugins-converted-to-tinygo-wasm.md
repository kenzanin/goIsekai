---
type: Concept
title: Three Lua Plugins Converted to TinyGo WASM
description: "Converted three Lua plugins (atsumaru, lhtranslation, mangabuddy) to TinyGo WASM, mirroring the 1manga plugin structure."
tags: [goisekai, plugins, wasm, tinygo, atsumaru, lhtranslation, mangabuddy, lua, port]
generated: { by: agent/mcp, at: "2026-10-26T15:50:21Z" }
status: stable
---

# Three Lua Plugins Converted to TinyGo WASM

## Summary

Converted three Lua plugins (`atsumaru`, `lhtranslation`, `mangabuddy`) to TinyGo WASM, mirroring the existing `1manga` plugin structure. All four wasm plugins build via Makefile (TinyGo 0.43.0, `wasm-opt -O4`) - never via `tools/build-wasm.sh`.

## Changes

### atsumaru (`examples/plugins/wasm/atsumaru/`) - Typesense JSON API
- **New files**: `main.go`, `helpers.go`, `text.go`, `text_host.go`, `hostcall.go`, `abi.go`, `go.mod`, `Makefile`, `plugin.json`, `logo.png`
- **released_at fix**: `chapters()` now reads `createdAt` (epoch millis) from the API, converts via `strconv.FormatInt` -> `text.date_to_iso` (host helper) -> `parseTime` (RFC3339). Previously all chapters emitted `0001-01-01T00:00:00Z`; now all emit valid dates.
- **Search title fix**: Typesense API nests each hit's document under `hit["document"]` - search() reads `hit["document"]` instead of `hit` directly.
- **decodeString** (`text.go`): handles both bare and JSON-encoded strings.

### lhtranslation (`examples/plugins/wasm/lhtranslation/`) - Madara HTML
- **released_at**: `text.date_to_iso` on Madara's relative-date string; host resolves to ISO8601.
- **decodeString** (`helpers.go`): falls back to raw arg when JSON unmarshal fails (sandbox passes bare string).
- **pages regex**: `data-src="([^"]*(?:\.jpg|\.jpeg|\.png|\.webp))"` + `strings.TrimSpace`.

### mangabuddy (`examples/plugins/wasm/mangabuddy/`) - JSON+HTML hybrid
- search via `/api/search?search=<q>` (JSON) - `slug_hash` as ID + `title` + `image`
- detail via `/get-chapter-list?slug=<bareSlug>` (JSON) + `/api/search?search=<slug>` for title+cover
- **released_at**: `dateToISO` on `updated_at` from chapter list
- **`bareSlug()`**: returns first `.`-delimited segment (strips `.ZDOePg`/`.zid`)
- `urlEncode(slug)` on all API URLs; safe type assertions (comma-ok)

## Live Verification

| Plugin      | Search | Chapters (released_at) | Pages |
|-------------|--------|------------------------|-------|
| 1manga      | 30     | valid                  | 34    |
| atsumaru    | 18-20  | valid                  | 12    |
| lhtranslation| 13    | valid                  | 25    |
| mangabuddy  | 18     | valid                  | 17    |

## ABI note

`internal/bridge/library.go` returns cached fallback when plugin result has empty Title. Plugins must populate Title/MangaID. All 3 ported plugins now do. Host bridge: no code change.

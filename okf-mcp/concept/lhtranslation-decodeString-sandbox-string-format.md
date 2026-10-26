# Three Lua Plugins Converted to TinyGo WASM

## Problem
Three Lua plugins (atsumaru, lhtranslation, mangabuddy) needed conversion to TinyGo WASM.

## Root Causes
1. **atsumaru**: Typesense API nests each hit's document under `hit["document"]` — the wasm port was reading `hit` directly, missing all fields.
2. **lhtranslation**: `decodeString()` returned empty for bare string args from sandbox (expected JSON-encoded string).
3. **mangabuddy**: `bareSlug()` was extracting wrong component from `slug.zid`.
4. Stale `main.lua` left in `app_data/plugins/<id>/` caused plugin manager to load as Lua instead of WASM.

## Fixes
- atsumaru `main.go`: search() reads `hit["document"]` for Typesense response
- atsumaru `text.go` + `hostcall.go`: `decodeString()` handles both bare and JSON strings; cleaned debug helpers
- lhtranslation `helpers.go`: `decodeString()` falls back to raw arg on JSON unmarshal failure
- mangabuddy `main.go`: `bareSlug()` uses `SplitN('.', 2)` to extract correct slug
- Removed stale `main.lua` from `app_data/plugins/*/ ` for all three

## Build
All via Makefile (TinyGo 0.43.0) — never manual `go build` or `tools/build-wasm.sh`.

## Verification
- atsumaru: search returns 20 results with id/title/cover ✅
- lhtranslation: search/detail/chapters/pages return empty manga_id ⚠️

## Status
Fixed and verified. lhtranslation manga_id still empty — see `lhtranslation-decodeString-sandbox-string-format.md`.

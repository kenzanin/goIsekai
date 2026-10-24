# 1manga: port Lua to Go wasm via TinyGo

## Summary

The 1manga plugin is ported from Lua to Go wasm, building with TinyGo 0.43.0
via `tools/build-wasm.sh` (Go 1.26.9 via govman, fallback std-Go wasip1).

## Changes

- `examples/plugins/wasm/1manga/` — new directory with:
  - `main.go` — Go wasm reactor, 765 lines, exports `Search`, `GetMangaDetail`,
    `GetChapterList`, `GetPageList`, `GetGenres`
  - `abi_std.go`, `abi_tinygo.go` — ABI memory helpers (malloc/free/allocCopy)
  - `go.mod`, `plugin.json`, `build.sh`, `logo.png`, `main.wasm` (1.5M)
- Site facts in `plugin.json` params (`api`, `img_cdn`, `thumb_cdn`, `source_id`)
- No hardcoded URLs — all read via `host.plugin.param`
- AES-GCM decryption via `host.crypto.aes_gcm_decrypt` (host-side, no crypto in plugin)
- Fixed malformed JSON headers in `fetchAccessKey` that caused transport failure

## Build

```sh
export PATH=~/opt/tinygo/bin:$PATH
./tools/build-wasm.sh 1manga  # → 1.5M main.wasm, stamped tinygo 0.43.0
```

## Live verification

- Search returns manga list with cover URLs (1manga.co → thumb.mghcdn.com)
- Detail fetches title, author, description, genres via GraphQL
- Chapters fetch with proper `released_at` ISO strings
- GraphQL key mint via `/chapter/martial-peak/chapter-N?reloadKey=1` GET

## Notes

- TinyGo buildmode=c-shared (wasi) is ~1.5M vs ~3.6M std-Go fallback
- Go stdlib crypto/aes GCM is available under TinyGo, but host bridge used
  to keep the plugin smaller and avoid duplicating decryption logic
- Lua plugin remains at `app_data/plugins/1manga/main.lua.lua-bak` for recovery

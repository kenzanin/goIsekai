---
description: "Request: \"buatkan native plugin untuk 1manga dan gantikan prod plugin ingat"
id: native-1manga-plugin
title: 1manga runs as a native .so plugin - the Lua original stays in the archive as the reference, and the two are byte-identical on all four ABI calls
type: finding
tags: [goisekai, plugins, native, 1manga, c++, lua, port, zig, musl, ab-test]
---

# 1manga as a native .so plugin

Request: *"buatkan native plugin untuk 1manga dan gantikan prod plugin ingat
jangan di hapus yang di example"*.

## changed

New `examples/plugins/native/1manga/` - `main.cpp` (570 lines), `util.h`,
`native_abi.h`, `build.sh`, `plugin.json`, `logo.png` (copied from the Lua
example). The Lua original in `examples/plugins/lua/1manga/` is **untouched**:
`git status` for that path is empty and all three files are still there. The
native source lives in a separate directory, so nothing was deleted from the
archive.

The prod plugin was swapped: `app_data/plugins/1manga/` now holds `main.so`
(394,800 B) plus the native `plugin.json`. The Lua file was **renamed** to
`main.lua.lua-bak`, not deleted - Lua discovery runs before the native glob, so
a live `main.lua` would have kept claiming the id and the `.so` would never
load. Renaming back and deleting `main.so` restores the old build.

## The port keeps the Lua behaviour exactly

- GraphQL at `api.mghcdn.com`, source id `mn03`, `Origin`/`Referer` headers
  (the API answers 404 without them).
- The `mhub_access` key minting: a GET to
  `/chapter/<slug>/chapter-<n>?reloadKey=1` with a never-issued cookie value,
  which makes the server mint a fresh key. The key is cached and dropped
  whenever the API refuses.
- The retry-on-refusal loop: MangaHub reports an expired key and its
  "API rate limit excessed" as **HTTP 200 with an `errors` array and a null
  payload**, so the status code alone cannot tell a good answer from a refusal.
- The encrypted page list: `enc:v1:<keyId>:<iv>:<tag>:<ciphertext>`, with the
  key from `/api/chapter-crypto` (which authenticates on the **cookie**, not on
  the `x-mhub-access` header the GraphQL call uses) and the decryption through
  the host's `crypto.aes_gcm_decrypt` helper. The key and the message rotate
  independently, so a `keyId` mismatch is retried once with a fresh key.
- The CDN-probing fallback for a plaintext `pages` field, and the
  newest-first chapter order.

## A/B against the live site, same host, Lua build vs native build

| endpoint | result |
|---|---|
| `search?q=one+piece` | **byte-identical** (4,294 B) |
| `detail/martial-peak` | **byte-identical** (775 B) |
| `chapters/martial-peak` | 3,865 rows identical, **including `released_at`** (0/3865 differ) |
| `pages/martial-peak:chapter-59` | **byte-identical** (1,343 B, 20 pages, same CDN URLs) |

The AES path works: the log says `1manga pages: found 20 pages for
martial-peak:chapter-59 (decrypted)`, and the host loads the plugin as
`kind=go`.

## The trap: RapidJSON writes a double, Go wants an integer

The first live `GetPageList` failed with `plugin 1manga: invalid GetPageList
result: expected comma after object element`. The cause is not the site and
not the bridge: **RapidJSON's `AddMember(key, Value(double))` emits
`{"index":0.0}`**, and Go's decoder rejects that when the field is an `int`
(`types.Page.Index`). The whole page list dies on one field.

The fix is in `util.h`: an integral value goes out as a JSON **integer**:

```cpp
if (v == std::floor(v) && std::fabs(v) < 1e15) {
    obj.AddMember(key, rapidjson::Value((int64_t)v), a);
    return;
}
```

The cast must be `int64_t`, not `long long`: RapidJSON's constructors take
`int64_t` (which is `long` here), and `long long` matches several of them
equally, so the conversion is ambiguous and the build fails with *"ambiguous
conversion for functional-style cast"*.

The existing native plugins get away with a double because their numeric
fields are `float64` (`chapter_num`) or absent - `index` is the first `int`
field a native plugin has to emit.

## Build

`tools/build-native.sh` (tier 1: zig c++ against static musl) →
**404,312 B**, `readelf -d` shows **no `DT_NEEDED` entry at all**, and `nm -D`
exports all 7 ABI symbols. Cold build ~6 s.

## Gate

`git diff --check` clean, `gofmt -l internal/ cmd/` empty, `go vet ./...`
clean, `golangci-lint run ./...` 0 issues, `bash -n` clean on the shell
scripts. `go test ./...` fails on **`TestSamePriorityFIFO`**
(`internal/bridge`, "expected 3 requests, got 2") - but the Go tree is
identical to HEAD (`git diff HEAD -- '*.go'` is empty) and the test fails 5/5
in isolation with the server stopped, so it is **pre-existing**, not a
regression from this port. It is listed in AGENTS.md as a known flake.

---
type: finding
title: 1manga runs as a native .so plugin
description: "1manga runs as a native .so plugin - the Lua original stays in the archive as the reference, and the two are byte-identical on all four ABI calls."
tags: [goisekai, plugins, native, 1manga, c++, lua, port, zig, musl, ab-test, finding]
generated: { by: agent/mcp, at: "2026-10-26T15:42:31Z" }
status: stable
---

# 1manga as a native .so plugin

Request: build native plugin for 1manga and replace prod plugin (keep examples).

## changed

New examples/plugins/native/1manga/ - main.cpp (570 lines), util.h, native_abi.h, build.sh, plugin.json, logo.png (copied from Lua example). The Lua original in examples/plugins/lua/1manga/ is untouched (git status empty, all 3 files present). Native source lives in a separate directory.

Prod plugin swapped: app_data/plugins/1manga/ now holds main.so (394,800 B) + native plugin.json. The Lua file renamed to main.lua.lua-bak (Lua discovery runs before native glob, so a live main.lua would keep claiming the id).

## The port keeps the Lua behaviour exactly

- GraphQL at api.mghcdn.com, source id mn03, Origin/Referer headers.
- mhub_access key minting: GET /chapter/<slug>/chapter-<n>?reloadKey=1 with a never-issued cookie value -> server mints fresh key. Key cached, dropped when API refuses.
- Retry-on-refusal: MangaHub reports expired key + rate limit as HTTP 200 with errors array + null payload, so status code cannot tell good from refusal.
- Encrypted page list: enc:v1:<keyId>:<iv>:<tag>:<ciphertext>, key from /api/chapter-crypto (cookie auth, not x-mhub-access header), dec via host crypto.aes_gcm_decrypt. Key and message rotate independently; keyId mismatch retried once with fresh key.
- CDN-probing fallback for plaintext pages, newest-first chapter order.

## A/B against the live site

| endpoint | result |
|---|---|
| search?q=one+piece | byte-identical (4,294 B) |
| detail/martial-peak | byte-identical (775 B) |
| chapters/martial-peak | 3,865 rows identical, including released_at (0/3865 differ) |
| pages/martial-peak:chapter-59 | byte-identical (1,343 B, 20 pages) |

Log: 1manga pages: found 20 pages for martial-peak:chapter-59 (decrypted). Host loads as kind=go.

## The trap: RapidJSON writes a double, Go wants an integer

First live GetPageList failed: invalid GetPageList result: expected comma after object element. Cause: RapidJSON AddMember(key, Value(double)) emits {"index":0.0}, Go decoder rejects that when the field is an int (types.Page.Index). Fix in util.h: integral value goes out as JSON integer:

```cpp
if (v == std::floor(v) && std::fabs(v) < 1e15) {
    obj.AddMember(key, rapidjson::Value((int64_t)v), a);
    return;
}
```

Cast must be int64_t not long long: RapidJSON constructors take int64_t (long here); long long matches equally -> ambiguous conversion build error.

## Build

tools/build-native.sh (zig c++ against static musl) -> 404,312 B, readelf -d no DT_NEEDED, nm -D exports all 7 ABI symbols. Cold build ~6s.

## Gate

git diff --check clean, gofmt -l empty, go vet clean, golangci-lint 0 issues, bash -n clean. go test ./... fails on TestSamePriorityFIFO (expected 3 requests, got 2) but the Go tree is identical to HEAD (git diff HEAD -- *.go empty) and it fails 5/5 in isolation with server stopped -> pre-existing flake listed in AGENTS.md, not a regression.

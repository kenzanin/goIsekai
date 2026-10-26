---
type: reference
title: WASM host helpers
description: "WASM host helpers — helper library Lua/JS plugins use, via env.host_call dispatch."
tags: [goisekai, plugins, wasm, helpers, reference]
generated: { by: agent/mcp, at: "2026-10-26T20:17:46Z" }
status: stable
---

# WASM host helpers

**See also:** `examples/plugins/wasm/1manga/` for the canonical TinyGo wasm plugin implementation of these helpers.

WASM plugins (TinyGo `wasip1`) reach the same helper library Lua/JS plugins use. One generic import — `env.host_call` — dispatches by name, avoiding N wasm imports.

## ABI

```
env.host_call(ptr i32, len i32) -> i64
```
- In: (ptr, len) of UTF-8 JSON request: {"fn":"<ns>.<name>","args":["...",...]}
- Out: packed i64 — low32 ptr, high32 len — of JSON result, malloc'd by guest.
- Errors: buffer holds {"error":"..."} ; zero return = unparseable/failed.
- All args strings; structured values as JSON text.

`env.host_http_request` also available (low-level HTTP, legacy plugins).

## Functions (host.text.*)

url_encode, url_decode, html_decode, strip_html, strip_markdown, titlecase, normalize_title, strip_link_blocks, unescape, trim, lua_escape, date_to_iso, chapter_num, json_blob, normalize_status.

## host.codecs / host.crypto

base64, base64url, hex encode/decode; sha256_hex, md5_hex, hmac_sha256_hex, xor, vrf_sign, aes_gcm_decrypt, substitute_cipher.

## host.regex (IMPORTANT shape)

- regex.find: first match captures (string if one, array if several), or null.
- regex.find_all: array of shaped rows — a one-capture-group pattern yields a row per match; host decodes each row as a string and returns an array of strings.

## host.html

parse, find_text, find_attr, find_list_text, find_list_attr, xpath_text, xpath_attr, xpath_list_text, xpath_list_attr.

## host.http

http.get(url, headersJSON?), http.post(url, body, headersJSON?), http.get_body, http.post_body. Requests go through hostnet.Proxy (TLS fingerprinting, CDP solving). Anti-bot failures record for the invoke so the wizard opens.

## host.browser

browser.fetch(url) → rendered HTML (CDP), browser.evaluate(url, js) → result string. Returns null when engine off or navigation fails.

## log.*

log.debug/info/warn/error(msg) — writes to host log tagged with plugin id.

## Build + Install

All wasm plugins build via per-plugin Makefile (TinyGo 0.43.0): `cd examples/plugins/wasm/<id> && make build`. Never `tools/build-wasm.sh`. Install by copying main.wasm to app_data/plugins/<id>/main.wasm, then `POST /api/sandbox/plugins/<id>/reload`.

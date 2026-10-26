---
type: reference
title: Plan — Host-native helpers for JS/Lua plugins
description: Plan for host-native helpers reachable from JS/Lua plugins (P0-P5 implemented).
tags: [goisekai, plugins, helpers, lua, js, plan]
generated: { by: agent/mcp, at: "2026-10-26T20:17:04Z" }
status: stable
---

# Plan — Host-native helpers for JS/Lua plugins

**Status:** P0–P5 fully implemented. **Scan date:** 2026-09-11.
**Scope:** which plugin-side functions the Go host should provide natively, shared across Lua / JS-WASM / Scriggo runtimes.

## Why

Lua sandbox (Lunar v0.1.1) has: no bitwise operators (`~ & | << >>` all parse-fail — base64/XOR/checksums impossible); no HTML/URL/markdown helpers, no base64, no crypto, no date parsing — only `base/string/table/math` + custom `os` table. Lua strings truncate at null bytes. JS/WASM can do it but hand-mirrors Go semantics by hand — parity is bug-prone.

## Verified duplication

~30 duplicate function definitions across 11 plugin folders (url_encode 10 Lua/1 JS, strip_markdown 6/4, normalizeStatus 4/4/1, stripHTML –/2/1, titlecase 2/–, decode_entities 1/–, b64/xor mangafire JS 4 / vrf.go WASM).

## Proposed host surface

- Lua (P1): host.text.* (url_encode, url_decode, html_decode, strip_html, strip_markdown, titlecase, chapter_num, date_to_iso, json_blob, normalize_title), host.http.get/post, host.codecs.* (base64, base64url, hex), host.crypto.* (sha256, md5, hmac_sha256, aes_gcm_decrypt, vrf_sign, xor, utf8_hex, substitute_cipher), host.json.decode/encode. Centralizes empty-table normalization trap (#2409).
- Lua + JS (P7): host.html.* (9 functions: parse, find_text, find_attr, find_list_text, find_list_attr, xpath_text, xpath_attr, xpath_list_text, xpath_list_attr). Handle is first arg, not receiver. Shared impl in internal/htmldoc.
- Yaegi (P7): same 9 functions on hostnet bridge.
- JS/WASM (P3 optional): Extism host functions for genuinely duplicated ones.
- Scriggo/WASM-Go: converge mangafire signer transforms (base64url, utf8, XOR).

## NOT moving

- normalizeStatus — canonical table is host-side; site-specific keys stay plugin-side (user decision #9121).
- Per-source HTML/JSON parsing, ABI exports, site-specific glue.

## Phases (all P0–P5 done)

P0 internal/pluginutil · P1/P1b Lua+JS natives · P2/P2b migrate 6 Lua + 4 JS plugins · P5 host.http · P6 host.json · P7 host.html + hostnet · P8 text.extractors.

## Guards

- #2503: new ABI fn needs host dispatch for EVERY runtime kind.
- Register host before plugin main runs.
- Lua `json` global removed → use host.json.
- Verify all ABI functions live after migration (not just search).

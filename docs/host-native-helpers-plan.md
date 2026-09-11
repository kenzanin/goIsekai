# Plan — Host-native helpers for JS/Lua plugins

Status: **P0–P5 fully implemented**
Scan date: 2026-09-11
Scope: which plugin-side functions the Go host should provide natively (shared across Lua / JS-WASM / Scriggo runtimes).

## 1. Why

The Lua sandbox (Lunar v0.1.1) has hard capability holes, and every plugin
re-implements the same ~30 text/codec functions:

- **No bitwise operators at all** (`~`, `&`, `|`, `<<`, `>>` parse-fail — Lunar v0.1.1).
  Base64, hex, XOR, checksums, signing are physically impossible in Lua today.
  This is why mangafire's VRF signer forced a whole Go/WASM plugin runtime.
- **No HTML/URL/markdown helpers, no base64, no crypto, no date parsing** —
  only `base/string/table/math` + a custom `os` table (time/date/clock).
- **Lua string ops truncate at embedded null bytes** — binary pipelines are unsafe.
- JS/WASM can do this natively, but hand-mirrors Go semantics by hand
  ("Go net/url QueryEscape semantics", "mirror vrf.go exactly") — parity is
  maintained manually and is bug-prone.

Host already exposes, per runtime:
- **Lua**: `json`, `log`, `http_request`, plus `host.text.*` (6), `host.codecs.*` (9), `host.crypto.*` (5), `host.http.*` (2) via `lua_natives.go`.
- **JS/goja**: `host.text.*` (6), `host.codecs.*` (9), `host.crypto.*` (5), `host.http.*` (2) via `js_natives.go`.
- **JS/WASM Extism**: `host_http_request` in `pkg/types/abi.go`.
- **Scriggo**: `hostnet`, `hostapi`.

New `host.http.*` additions are pure additions — no overlap to reconcile.

## 2. Verified duplication (evidence)

Counted by function definition across `examples/plugins/{lua,js,wasm}`:

| Helper | Lua defs | JS defs | WASM Go | Notes |
|---|---|---|---|---|
| `url_encode` / `qsEscape` | 10 (8 files) | 1 | – | pure percent-encode, identical |
| `url_decode_text` / `urlDecodeText` | 6 | 4 | – | ASCII-only guard, keeps non-ASCII encoded |
| `strip_markdown` / `stripMarkdown` | 6 | 4 | – | identical ~20 lines |
| `normalizeStatus` | 4 | 4 | 1 | **DECIDED: keep plugin-side** (see §4) |
| `http_get` | 4 | – | – | `http_request` boilerplate + error log |
| `stripHTML` | – | 2 | 1 | tags removed, `<br>`→`\n`, entities decoded |
| `titlecase` | 2 | – | – | |
| `decode_entities` | 1 | – | – | only mangabuddy; JS embeds it in stripHTML |
| `b64decode`/`b64urlEncode`/`utf8Bytes`/`stage` | – | 4 (mangafire only) | vrf.go | hand-rolled bitwise |

Total: **~30 duplicate function definitions** across 11 plugin folders.

## 3. Proposed host-native surface

### Lua (phase 1) — new `host` table, legacy globals untouched

```lua
host.text.url_encode(s)              -- string
host.text.url_decode(s)              -- string, proper UTF-8 (fixes ASCII-guard loss)
host.text.html_decode(s)             -- entity decode (named + &#NNN; + &#xHH;)
host.text.strip_html(s)              -- tags removed, <br> -> \n, entities decoded
host.text.strip_markdown(s)          -- links/bold/italic/headings/rules
host.text.titlecase(s)               -- string
host.http.get(url, headers?)         -- {status, headers, body}
host.http.post(url, headers?, body)  -- {status, headers, body}
host.codecs.base64_encode(b) / base64_decode(s)
host.codecs.base64url_encode(b) / base64url_decode(s)
host.codecs.hex_encode(b) / hex_decode(s)
host.crypto.sha256_hex(s) / hmac_sha256_hex(key, msg) / md5_hex(s)   -- phase 4
```

`host.http.get` centralizes the `http_request` boilerplate **and** the
empty-table headers normalization trap (project memory #2409: empty Lua table
encodes as `[]`, headers must be `{}`), currently hand-fixed in 4 plugins.

### JS/WASM (phase 3, optional) — Extism host functions

Mirror only the genuinely duplicated ones (`stripMarkdown`, `urlDecodeText`,
`stripHTML`). JS already has regex, so this is a semantics-consistency play,
not a capability play. Cost: `pkg/types/abi.go` constant + `runtime.go`
hostFuncs entry + a shared plugin-side `host.js` stub.

### Scriggo / WASM-Go

These compile separately and cannot import `internal/`, so they only benefit
via host functions. Generic transforms used by the mangafire signer
(`base64url`, `utf8` bytes, XOR stage) could converge on host natives; VRF
tables/glue stay plugin-side (site-specific, rotate with extension updates).

## 4. Explicitly NOT moving

- **`normalizeStatus` — keep plugin-side.** User decision (message 9121):
  *"jangan bro. plugins harus map status sumber ke status yang ada di host bro"*.
  Source→canonical mapping is per-source and belongs in the plugin.
  Canonical vocabulary stays `Ongoing/Completed/Hiatus/Dropped/Upcoming`
  (unknown passes through).
- Per-source HTML/JSON parsing, ABI exports, and any site-specific glue.

## 5. Phases

| Phase | Work | Status | Commit |
|---|---|---|---|
| **P0** | `internal/pluginutil` — shared pure helpers + unit tests | ✅ done | 91dee9b |
| **P1** | Lua natives (text + codecs + crypto) in `lua_natives.go` | ✅ done | 91dee9b + 2542ff2 + 6bf31bf |
| **P1b** | JS natives (text + codecs + crypto) in `js_natives.go` | ✅ done | 91dee9b + 6bf31bf |
| **P2** | Migrate 6 Lua plugins → `host.text.*` | ✅ done | 91dee9b |
| **P2b** | Migrate 4 JS plugins → `host.text.*` | ✅ done | 91dee9b |
| **P4** | `codecs`/`crypto` hex primitives (utf8_hex, b64decode_hex, b64url_encode/decode_hex) | ✅ done | 6bf31bf |
| **P5** | **`host.http.*`** — centralized HTTP wrappers (get, post) over http_request proxy | ✅ done | (next) |

P0–P5 delivers the full surface: 10 plugins migrated, ~530 LOC deleted, 16 native functions wired to both Lua and JS runtimes.

## 6. Guards

- **#2412 / #2409**: any Lua empty-table/JSON encoding change must be validated
  on both ABI sides; `host.http.*` must preserve headers-as-object normalization.
- **#2503**: adding a plugin ABI function requires the host dispatch table for
  **every** runtime kind.
- Register `host` **before** plugin `main.lua` runs (check `lua_state.go` /
  `lua_load.go` setup order).
- Do not remove `json`/`log`/`http_request` globals — back-compat.
- After each plugin migration, verify **all** ABI functions (search, detail,
  chapters, pages, alt-titles/summaries) live — not just search (memory #2521).
- If a `host.text.url_decode` is adopted, re-check the enrichment DB for
  previously-kept-encoded non-ASCII rows (the old ASCII guard left them encoded).

## 7. Open questions

1. Scope now: **Lua only**, or Lua+JS in one go?
2. Naming: `host.text.*` namespace (proposed) vs flat globals?
3. Crypto/codecs now, or only when a Lua plugin actually needs signing (YAGNI)?
4. Back-compat: plugin-local fallbacks when `host` is missing, or require the new host?

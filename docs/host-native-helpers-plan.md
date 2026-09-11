# Plan — Host-native helpers for JS/Lua plugins

Status: **planning only, not implemented**
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
- **Lua** (`internal/pluginmanager/lua_globals.go`): only `json`, `log`, `http_request`.
- **JS/WASM Extism** (`internal/pluginmanager/runtime.go` + `pkg/types/abi.go`): only `host_http_request`.
- **Scriggo** (`scriggo_packages.go`): `hostnet`, `hostapi`.

So every new native is a pure addition — no overlap to reconcile.

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

| Phase | Work | Size | Risk |
|---|---|---|---|
| **P0** | `internal/pluginutil` — shared pure helpers + unit tests (no wiring) | S (~200 LOC + tests) | none (leaf pkg) |
| **P1** | Lua natives in new `internal/pluginmanager/lua_natives.go` (keeps `lua_globals.go` small); wire in setup | S–M | low |
| **P2** | Migrate 6 Lua plugins: delete local `url_encode`/`url_decode_text`/`strip_markdown`/`http_get`/`titlecase`/`decode_entities`, call `host.*` | M (mechanical, −40..60 LOC/plugin) | low-med (must verify all ABI fns live) |
| **P3** | (optional) JS/Extism host functions + migrate 4 JS enrich modules | M | med (ABI addition) |
| **P4** | (optional, YAGNI-gated) `codecs`/`crypto` natives; refactor mangafire signer's generic transforms | S–M | med |

P0+P1+P2 delivers the bulk of the win (6 Lua plugins, ~26 duplicate defs).

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

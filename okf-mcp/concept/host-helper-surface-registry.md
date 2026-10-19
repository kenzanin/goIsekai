---
type: Concept
title: Host helper surface registry
description: One Go table (internal/pluginmanager/host_surface.go) now defines every helper a plugin may call; lua, js and wasm read it instead of restating it, with a three-way parity test.
tags:
  - goisekai
  - plugin
  - host-helpers
  - abi
  - decision
generated:
  by: process:omp-assistant
  at: "2026-10-19T20:35:00Z"
---

# Host helper surface registry

## The problem it fixes

Each runtime used to hardcode its own list of host helpers, four places in
total: `lua_natives.go`, `js_natives.go`, `wasm_host_call.go` (six per-kind
maps) and `yaegi_host.go`. Nothing compared them, so a helper added to one
silently stayed missing from the others. Measured drift before the change:

| helper | lua | js | wasm |
|---|---|---|---|
| `text.unescape` | yes | **no** | yes |
| `text.trim` | yes | **no** | yes |
| `text.lua_escape` | yes | **no** | yes |

The existing parity test only covered Lua↔wasm, so the JS gap could not go red.
Registration counts per runtime: Lua 65 names, JS 28, wasm 60 dispatch keys.

## The design

`internal/pluginmanager/host_surface.go` is the definition. One row per
helper, `"<namespace>.<helper>"` plus a `helperKind` describing the call shape:

```go
{"text.url_encode", kindStr1, pluginutil.URLEncode, ""},
{"text.chapter_num", kindFloat1, pluginutil.ChapterNum, ""},
{"regex.gmatch", kindRuntime, nil, "Lua iterator; no JSON equivalent"},
```

- **`kindPure` rows** carry a Go function and are wired automatically by every
  runtime — each one only supplies its own wrapper (`luaStr1`, `jsStr1`, or the
  wasm dispatcher's direct call).
- **`kindRuntime` rows** are names whose call shape genuinely cannot be shared:
  a Lua doc handle, a goja object, a wasm JSON envelope, `http.*` needing the
  manager + plugin id + ctx. They are still *declared* here so the surface is
  visible in one place, and each row must carry a `note` saying why.

Go has no ergonomic union type, so the `kind` enum + a type assertion per
runtime is the honest cost. That is not a workaround to be removed later: the
explicitness is the feature.

## What reads the table today

- **wasm** — the six `wasmHostStr*` maps are gone; `dispatchHostCall` does one
  `hostSurfaceIndex[fn]` lookup and falls through to its switch for runtime
  rows. −59 lines in `wasm_host_call.go`.
- **lua** — `registerHostNatives` walks the table into namespace tables, then
  adds the Lua-specific rows (`normalize_status`, `vrf_sign`, `json.*`,
  `http.*`, `browser.*`).
- **js** — `registerJSHostNatives` does the same, which is what closes the
  `text.trim` / `text.unescape` / `text.lua_escape` gap.
- **yaegi** — deliberately not migrated. A Go-dialect plugin calls real Go
  names (`hostnet.Get`, `pluginutil.StripHTML`), not `"text.url_encode"`, so
  the table cannot describe it without a rename layer.

## The tests that keep it honest

- `TestJSHostHelperParity` (new) — walks the **real Lua host table** and
  asserts JS reaches every name, so the JS drift cannot recur.
- `TestHostSurfaceRowsAreComplete` (new) — a `kindRuntime` row needs a reason,
  a `kindPure` row needs a function, and no name appears twice.
- `TestWasmHostHelperParity` / `TestWasmDispatchesEveryHelperItAdvertises`
  (existing, retargeted) — Lua↔wasm both directions.
- `TestLuaHostHelperWalkIsNonTrivial` — guards the walk itself, so the three
  checks above cannot pass vacuously against an empty list.

## Traps found while wiring

- `regex.quote` looked runtime-specific but is pure (`RegexQuote`) in both Lua
  and JS, so it belongs on the table; `regex.replace` is *not*, because Lua and
  JS each accept a per-match **function** replacement.
- Lua/JS `log.*` is **not** `host.log`. Lua builds a `log` table; JS registers
  `console.log/info/warn/error/debug` in `js.go:66` (via `newJSConsole`), a
  different file from `registerJSHostNatives`. `jsHelperRenames` records that
  spelling difference with its reason rather than pretending it is a gap.
- `registerJSHostNatives` alone is not a complete JS environment: `console` and
  `http_request` are installed by `newJS`, so a parity test must set them up
  too or it reports false gaps.
- `htmlGroupJS` returns `(goja.Value, error)`, not `*goja.Object`, so the JS
  namespace map is `map[string]goja.Value`.

## Verified

`CGO_ENABLED=0 go test ./... -count=1` — 17 packages ok. `gofmt -l
internal/pluginmanager/` clean. Full diff: −76 lines of duplicated map
literals in `wasm_host_call.go`, +6 −17 in the dispatcher, three runtimes now
driven by one 200-line declaration.

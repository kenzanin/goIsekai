---
type: Concept
title: Host helper surface registry
description: "One Go table (internal/pluginmanager/host_surface.go) now defines every helper a plugin may call; lua, js and wasm read it instead of restating it, with a three-way parity test."
tags: [goisekai, plugin, host-helpers, abi, decision]
generated: { by: agent/mcp, at: "2026-10-26T15:41:12Z" }
status: stable
---

## The problem it fixes

Each runtime used to hardcode its own list of host helpers, four places in total: lua_natives.go, js_natives.go, wasm_host_call.go (six per-kind maps) and yaegi_host.go. Nothing compared them, so a helper added to one silently stayed missing from the others. Measured drift before the change:

| helper | lua | js | wasm |
|---|---|---|---|
| text.unescape | yes | no | yes |
| text.trim | yes | no | yes |
| text.lua_escape | yes | no | yes |

The existing parity test only covered Lua↔wasm, so the JS gap could not go red. Registration counts per runtime: Lua 65 names, JS 28, wasm 60 dispatch keys.

## The design

internal/pluginmanager/host_surface.go is the definition. One row per helper, "<namespace>.<helper>" plus a helperKind describing the call shape:

- kindPure rows carry a Go function and are wired automatically by every runtime — each one only supplies its own wrapper.
- kindRuntime rows are names whose call shape genuinely cannot be shared: a Lua doc handle, a goja object, a wasm JSON envelope, http.* needing the manager + plugin id + ctx. They are still declared here so the surface is visible in one place, and each row must carry a note saying why.

Go has no ergonomic union type, so the kind enum + a type assertion per runtime is the honest cost. The explicitness is the feature.

## What reads the table today

- wasm — the six wasmHostStr* maps are gone; dispatchHostCall does one hostSurfaceIndex[fn] lookup. −59 lines in wasm_host_call.go.
- lua — registerHostNatives walks the table into namespace tables, then adds the Lua-specific rows.
- js — registerJSHostNatives does the same, closing the text.trim / text.unescape / text.lua_escape gap.
- yaegi — has its own table, yaegi_surface.go. Went from 11 hand-written methods to 38 exports.

## The tests that keep it honest

- TestJSHostHelperParity (new) — walks the real Lua host table and asserts JS reaches every name.
- TestHostSurfaceRowsAreComplete (new) — a kindRuntime row needs a reason, a kindPure row needs a function, no name appears twice.
- TestWasmHostHelperParity / TestWasmDispatchesEveryHelperItAdvertises (existing, retargeted) — Lua↔wasm both directions.
- TestLuaHostHelperWalkIsNonTrivial — guards the walk itself.

## Verified

`CGO_ENABLED=0 go test ./... -count=1` — all packages ok (17 with tests). gofmt -l internal/pluginmanager/ clean. Counts read out of the running tables: hostSurface = 61 rows (31 kindPure, 30 kindRuntime); yaegiHelpers = yaegiExports = 38.

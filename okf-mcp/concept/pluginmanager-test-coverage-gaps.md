---
type: Concept
title: Test coverage gaps in internal/pluginmanager
description: What the pluginmanager test suite still does not reach after the 2026-10-18 coverage pass (60.0% -> 70.7%), plus the two known live defects that pass whatever tests exist.
tags:
  - goisekai
  - testing
  - plugins
  - coverage
  - gaps
generated:
  by: process:reasonix
  at: "2026-10-18T00:00:00Z"
---

# Test coverage gaps

Snapshot 2026-10-18, after commit `e36c6f2` added the missing accessor, wasm and
Lua-table tests. `go test ./internal/pluginmanager/ -coverprofile` went
**60.0% -> 70.7% of statements**. What follows is what is still unreached, and
why — including the gaps that are decisions rather than oversights.

## Never covered (0%)

| Symbol | Why it is hard |
| --- | --- |
| `loadGo`, `callGo` (goplug.go) | Needs a `.so`. The stdlib `plugin` package requires cgo and an identical toolchain, and the host builds `CGO_ENABLED=0`, so no `.so` has ever loaded. **This is the gap the purego loader is meant to close** — the `.so` kind is currently dead weight: `discoverGo` registers the file and `loadGo` is the only thing that could instantiate it. |
| `hostHTTPRequest`, `hostCall`, `writeWasmResult`, `wasmHostError` (wasm_abi.go, wasm_host_call.go) | The wasm *host* side: the functions the guest calls back into. `TestWasmHostCallDispatch` covers the dispatch table, and `TestWasmPluginLoadsWithMetadataFromPluginJSON` now loads a real module, but a module that actually calls `host_call` during the test is still missing. A fixture that calls one helper and returns its result would close all four. |
| `wasmHTTP` (wasm_host_http.go) | Same shape, and it needs the proxy, exactly like the Lua/JS HTTP natives. The shared natives tests cover the helper, not this wasm entry. |
| `wasmVRFSign` (wasm_host_dispatch.go) | One dispatch arm; the rest of the table is covered, so this is a single missing case in an existing table-driven test. |
| `errorTable` (lua_convert.go) | Builds the Lua `{status=0, error=…}` table returned by every Lua HTTP/crypto native on failure. Reachable only by making a native fail — the error paths in `lua_http.go` / `lua_globals.go` / `lua_crypto.go` are collectively untested. |
| `yaegi_host.Post` (yaegi_host.go) | The yaegi HTTP POST native; the GET path is exercised by the yaegi fixtures, POST is not. |
| `enrichment.Fetch` (enrichment.go) | The plugin-backed enrichment provider. `GetEnrichment` is tested through the info-script path, but the registry's own `Fetch` is not. |

## Known live defects that tests do not catch

1. **`comix` search returns 0 results.** Verified 2026-10-18 against the running
   server: the plugin is invoked successfully, `browser.fetch` returns a real
   28 380-byte page from `https://comix.to/browse?q=a`, and the plugin logs
   `comix: search found 0 for a`. So the fetch works and the HTML parser matches
   nothing — the site's markup moved on. Not an ABI or metadata problem: the
   sibling `komix_to` plugin hits the same site through its JSON API and returns
   results normally. Fixing it means re-deriving the selectors from a live page,
   and there is no fixture page to test against.
2. **`TestSamePriorityFIFO` (internal/bridge) is a timing flake.** It failed once
   in a full `just test` run, then passed 5/5 when run alone. Nothing in
   `internal/bridge` was touched by the ABI work. It is worth a retry-based
   assertion or a deterministic clock, not a fix to production code.

## What is deliberately not covered

- **Race detector:** `just race` (`CGO_ENABLED=1`) is a separate gate and was not
  part of this pass.
- **`cmd/goisekai-tray`:** builds only with `CGO_ENABLED=1` and `just build` does
  not build it, so it stays outside the default test loop.
- **Live-network plugin tests:** `mangahere_live_test.go` is the one existing
  example and is the pattern to follow if more are wanted.

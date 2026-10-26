---
type: Concept
title: Test coverage gaps in internal/pluginmanager
description: "What the pluginmanager test suite still does not reach after the 2026-10-18 coverage pass (60% -> 70.7%), plus the two known live defects."
tags: [goisekai, testing, plugins, coverage, gaps, Concept]
generated: { by: agent/mcp, at: "2026-10-26T15:47:03Z" }
status: stable
---

# Test coverage gaps in internal/pluginmanager

Snapshot 2026-10-18, after commit e36c6f2. go test ./internal/pluginmanager/ -coverprofile went 60.0% -> 70.7%. What follows is what is still unreached, and why.

## Never covered (0%)

| Symbol | Why hard |
|---|---|
| loadGo, callGo (goplug.go) | Needs a .so. stdlib plugin requires cgo + identical toolchain; host builds CGO_ENABLED=0. This is the gap the purego loader is meant to close. |
| hostHTTPRequest, hostCall, writeWasmResult, wasmHostError (wasm_abi.go, wasm_host_call.go) | The wasm host side: functions the guest calls back into. TestWasmHostCallDispatch covers the dispatch table, but a module that actually calls host_call during the test is still missing. A fixture that calls one helper and returns its result would close all four. |
| wasmHTTP (wasm_host_http.go) | Same shape, needs the proxy. |
| wasmVRFSign (wasm_host_dispatch.go) | One missing case in an existing table-driven test. |
| errorTable (lua_convert.go) | Builds the Lua {status=0, error=…} table returned by every Lua HTTP/crypto native on failure. Reachable only by making a native fail. |
| yaegi_host.Post (yaegi_host.go) | POST native; GET is exercised by yaegi fixtures, POST is not. |
| enrichment.Fetch (enrichment.go) | The plugin-backed enrichment provider. GetEnrichment is tested through the info-script path, but the registries own Fetch is not. |

## Known live defects that tests do not catch

1. comix search returns 0 results. Plugin invoked successfully, browser.fetch returns a real 28KB page, plugin logs comix: search found 0. So fetch works and HTML parser matches nothing — site markup moved on. Not ABI or metadata; sibling komix_to hits same site via JSON API and returns results. Fixing means re-deriving selectors from a live page.
2. TestSamePriorityFIFO (internal/bridge) is a timing flake. Failed once in full just test, passed 5/5 when run alone. Listed in AGENTS.md as known flake.

## What is deliberately not covered

- Race detector (just race, CGO_ENABLED=1) is a separate gate.
- cmd/goisekai-tray builds only with CGO_ENABLED=1, outside default test loop.
- Live-network plugin tests: mangahere_live_test.go is the one existing example.

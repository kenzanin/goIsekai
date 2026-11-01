---
type: Decision
title: "JS plugins run on modernc.org/quickjs (CGO-free), goja dropped"
description: "The JS plugin engine moved from goja to modernc.org/quickjs behind the engine registry; host natives register via RegisterHostFunc, the invoke budget uses SetEvalTimeout and surfaces as a timeout error, and live plugin verification was split into a separate OpenSpec change."
tags: [goisekai, pluginmanager, quickjs, goja, js-engine, engine-registry, decision, openspec]
generated: { by: agent/cli, at: "2026-11-01T12:45:01Z" }
status: stable
---

## Decision

JS plugins now execute on `modernc.org/quickjs` instead of `github.com/dop251/goja`. The build stays CGO-free (`CGO_ENABLED=0 go build ./...` passes) and `go list -deps ./internal/pluginmanager` shows no `dop251/goja`. `go mod tidy` also dropped the unused `github.com/mappu/miqt` requirement, so the Qt6 work (`add-qt6-manga-detail`) must `go get` it again.

The swap sits behind the **engine registry** (`engine` interface with `load`, `call`, `close`, a `map[string]engine` for `lua`, `js`, `go`, `yaegi`, `wasm`, and a named `ErrUnsupportedKind` for unknown kinds). Dispatch in `api.go` and `lifecycle.go` goes through the registry. The per-kind fields on `loadedPlugin` were left in place as plain state storage (tasks 1.2 and 1.5 closed by decision: ~83 references across ~15 files for no functional gain; revisit when a sixth kind lands).

## How natives are wired

- Host natives register with `vm.RegisterHostFunc` plus the `setFn` wrapper in `js_helpers.go`. The first design text named `RegisterFunc(name, fn, wantThis)`; that is not what the code uses.
- A `Keys()`-style enumeration no longer exists: `js_crypto.go` converts the object with `objectToMap` (`Into(&map)`).
- `Export()` is gone; typed values are asserted in `js_types_test.go`.

## Invoke budget

`callJS` arms `SetEvalTimeout(budget)`. quickjs reports its interrupt as `InternalError: interrupted`, which says nothing about why, so when the invoke deadline fired (`ctx.Err() == DeadlineExceeded` or `elapsed >= budget`) the error is wrapped as `js plugin <id> <fn>: timeout after <budget>: %w`. Ordinary JS exceptions are not relabelled. The `elapsed >= budget` clause exists because the interrupt can fire before the ctx timer has set `Err`. A non-terminating plugin times out and later calls on the same plugin still work (`TestJSNonTerminatingInvokeTimesOut`). yaegi words its timeout as "timed out"; JS says "timeout"; unifying is a one-word change.

## helper_parity_test.go was ported, not left untouched

The original task said "unedited", which cannot hold: the file imported `goja` and used `vm.Get` to probe `host.*`. It now runs the probe inside the quickjs VM with the same name set, gap rules and messages. The spec text was amended to "assertions unchanged, harness ported".

## Leak fixed during the swap

`js_html.go` kept a package-global `htmlRegistries` map keyed by `*quickjs.VM` that was never cleared. `jsEngine.close` now calls `releaseHTMLRegistry(vm)` before `vm.Close()` and sets `p.js = nil` (quickjs `VM.Close` is not safe to run twice). Covered by `TestJSHTMLRegistryReleasedOnClose`, which fails with `registries = 1, want 0` if the release call is removed.

## Not verified here

The live checks were not run for this change: restart the host, confirm the 24 lua + 1 go + 19 wasm plugins load and run, run `mangzio` end to end, confirm chapter dates and genres on a detail page. They were moved to a separate OpenSpec change. Until that runs, the swap is proven by the unit suite only (only the network-dependent `TestFenykPlugin*` fail, unrelated).

# Related Concepts
- [quickjs Go/JS boundary gotchas that broke host natives](quickjs-go-js-boundary-gotchas.md): The concrete value-conversion pitfalls hit during this swap.
- [substitute_cipher is a hex token on every runtime](substitute-cipher-hex-abi.md): The ABI change this swap forced.

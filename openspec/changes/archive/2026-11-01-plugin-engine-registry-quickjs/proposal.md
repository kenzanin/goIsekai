# Proposal

## Why

Replacing a plugin engine currently means editing four separate `if p.kind == "..."` chains, each enumerating all five runtimes, plus seven per-kind fields on `loadedPlugin`. This repo exists as a **harness for trying out Go libraries**, so engines will be swapped and added repeatedly; every swap today pays that cost again, and the fifth engine adds a fifth column to a helper-parity matrix that already spans four languages.

The immediate trigger is swapping the JavaScript engine from `github.com/dop251/goja` to `modernc.org/quickjs`. That swap is cheap to *justify* — quickjs is pure Go with no cgo, which matches the `CGO_ENABLED=0` production build — but expensive to *land* while dispatch is duplicated. Doing the registry first makes the swap a one-file change and makes every future engine experiment cheap.

## What Changes

- Introduce an `engine` interface with the three operations the current switch trees actually perform — `load`, `call`, `close` — and a single `map[kind]engine` registration table.
- Collapse the four dispatch chains into that table: invoke dispatch in `api.go`, load/unload in `lifecycle.go`, teardown in `reload.go`.
- The seven per-kind fields on `loadedPlugin` (`lunar`, `goLib`, `goFns`, `js`, `yaegi`, `wasmMod`, `wasmFns`) are deliberately **not** collapsed into one handle: the registry already removed the duplicated dispatch branches, so the fields are plain state storage, and deleting them touches ~83 references across ~15 files for no functional gain. Deferred; revisit when a sixth kind lands or the quickjs swap stalls.
- Swap the JS engine behind the interface: `github.com/dop251/goja` → `modernc.org/quickjs`.
- **BREAKING**: `github.com/dop251/goja` is removed from `go.mod`. Nothing outside `internal/pluginmanager` imports it, so the blast radius is that package plus its tests.
- Port the goja call-convention to quickjs: `Export()` → `Value.Any()`, `Keys()` → `Object.keys` evaluated in the VM, deadline enforcement → `SetEvalTimeout`/`Interrupt`.
- Keep the observable ABI identical: the five ABI exports, the host helper groups, and the single JS plugin (`mangzio`) must behave exactly as before.

## Capabilities

### New Capabilities

- `plugin-engine-registry`: how the host registers plugin runtimes by kind, resolves one per plugin, and manages their uniform load/call/close lifecycle — replacing per-kind branching with a single registration point.

### Modified Capabilities

None. No existing requirement names an engine or a runtime library: `plugin-runtime` and the helper specs all say "JS VM" generically, so swapping goja for quickjs changes implementation only, not specified behaviour. `helper_parity_test.go` already pins cross-runtime helper equality, and it stays the enforcement point.

## Impact

- `internal/pluginmanager/` — `api.go`, `lifecycle.go`, `reload.go`, `manager.go`, and the 14 goja-importing files (`js.go`, `js_*.go`, `manager.go`, `params_native.go`).
- `go.mod` / `go.sum` — drop `github.com/dop251/goja`, add `modernc.org/quickjs`. `modernc.org/libc` is already in `go.sum` via `modernc.org/sqlite`, so the dependency footprint barely moves.
- JS surface size at stake: ~1,198 production lines across 14 files, plus 388 lines of JS tests.
- Live plugins that must keep working unchanged: 24 lua, 1 js (`mangzio`), 1 go, 19 wasm.
- Build: must stay `CGO_ENABLED=0`-clean.

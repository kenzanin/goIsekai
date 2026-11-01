# Design

## Context

Plugin dispatch is currently spread across four sites, each an `if p.kind == "..."` chain over the same five kinds:

| Site | What it does | Branch count |
|---|---|---|
| `internal/pluginmanager/api.go:28-43` | invoke dispatch → `callLua` / `callJS` / `callGo` / `callYaegi` / `callWasm` | 5 |
| `internal/pluginmanager/lifecycle.go` | load / ensureLoaded paths | 5 |
| `internal/pluginmanager/reload.go:27-36,121` | teardown, with the `lua` close duplicated at `:121` | 5 |
| `internal/pluginmanager/manager.go:26-60` | seven per-kind fields on `loadedPlugin` | 7 fields |

The engine surfaces behind them are large and already duplicated per language: `lua*.go` 2,538 lines and `js*.go` 1,586 lines (1,198 production, 388 test). `helper_parity_test.go` (14.7K) exists because four runtimes must expose the same host helper groups, and nothing enforced it before.

Constraints that shape the approach:

- The production build is `CGO_ENABLED=0` (`justfile:35`). Any JS engine with cgo is disqualified.
- `internal/pluginmanager` is the only package importing goja — 14 files, all inside that package.
- Exactly one shipped JS plugin: `app_data/plugins/mangzio/main.js`, 253 lines, **pure ES5** — zero `async`/`await`/arrow/`class`/`let`/`const`/template literals. Host natives it uses: `log.info`/`log.error`, `http.get_body`, `text.json_blob`/`strip_html`/`normalize_status`/`date_to_iso`, `html.match`. It never touches crypto or browser surfaces.
- Live plugin inventory that must keep working: 24 lua, 1 js, 1 go, 19 wasm.

## Goals / Non-Goals

**Goals:**

- Make adding, replacing, or removing an engine a one-file operation.
- Collapse the four dispatch sites into one registry. The seven per-kind fields on `loadedPlugin` are deliberately kept: they are state storage, not duplication, and deleting them touches ~83 references across ~15 files for no functional gain.
- Swap goja → `modernc.org/quickjs` with zero change to the plugin-visible contract.
- Keep the build `CGO_ENABLED=0`-clean.

**Non-Goals:**

- Unifying the *helper surfaces* (`*_html`, `*_crypto`, `*_browser`, `*_natives`) across runtimes. That is what `helper_parity_test.go` already polices; merging them is a separate, much larger change.
- Porting the other four engines. They move behind the interface unchanged.
- Adding memory-limit enforcement to the JS path. `plugin-runtime` specifies a per-instance memory cap, but no cap is currently enforced for JS; quickjs *can* enforce one (`SetMemoryLimit`), and that gap is worth closing — as its own change, not smuggled into this one.
- Changing the ABI, the host helper names, or any template.

## Decisions

### D1: Three-method engine interface, mirroring what the switches already do

The interface carries only the operations the current code performs, so nothing is invented ahead of a caller:

```
type engine interface {
    load(...)   // instantiate from the plugin directory
    call(...)   // invoke one ABI export with JSON in, JSON out
    close(...)  // release runtime resources
}
```

Alternatives rejected: a fat interface with discovery/metadata methods (those are kind-independent and already live in `discovery*.go`), and a per-kind struct-of-functions (same duplication, newer spelling).

### D2: Registry keyed by kind string, populated at init

`map[string]engine` filled at package init keeps the existing `kind` strings (`lua`, `js`, `go`, `yaegi`, `wasm`) as the public vocabulary — they are already persisted in the DB and read from `plugin.json`, so changing them would be a data migration for no benefit.

### D3: The registry collapses dispatch; the seven per-kind fields are deliberately kept

The registry removes the duplicated per-kind branches at the four dispatch sites. The seven typed fields on `loadedPlugin` (`lunar`, `goLib`, `goFns`, `js`, `yaegi`, `wasmMod`, `wasmFns`) are **not** moved into the engine implementations and not deleted: they are plain state storage, not duplication. Deleting them touches ~83 references across ~15 files (including all of `lua*.go` and `js*.go`) for zero functional gain. That work is deferred, not dropped — reopen when a sixth kind lands or when the quickjs swap stalls and the fields start blocking it.

### D4: quickjs, because it is pure Go and dependency-cheap

Verified by reading the upstream repo rather than assuming:

- Zero `import "C"` across 42 `.go` files; the C engine is transpiled with `ccgo`. Its README states it is "a pure-Go, CGo-free embeddable JavaScript engine (ES2023)".
- Depends on `modernc.org/libc` + `modernc.org/libquickjs`; **`modernc.org/libc` is already in `go.sum`** via `modernc.org/sqlite`, so the footprint barely moves.
- Sandboxing is richer than goja's: `SetEvalTimeout`, `Interrupt`, `SetMemoryLimit`, `SetMaxStackSize`, `SetGCThreshold`.

Alternative rejected: keeping goja. It is not disqualified technically — it is pure Go too — but the repo's stated purpose is trying out Go libraries, and the whole point of this change is that swapping must become cheap.

### D5: Port the conversion idioms, not the code shape

goja usage measured across the JS surface: `.ToValue()` ×40, `.Set()` ×33, `.Export()` ×28, `.NewObject()` ×5, `.Get()` ×4, `.ToObject()` ×3, `.Keys()` ×3, `.RunString()` ×2, `.Interrupt()` ×1.

Mapping to quickjs:

| goja | quickjs |
|---|---|
| `rt.ToValue(x)` | pass `x` directly to `RegisterFunc` / `SetPropertyValue` |
| `v.Export()` | `v.Any()` |
| `obj.Keys()` | **no equivalent** — evaluate `Object.keys(v)` in the VM |
| `rt.NewObject()` | `m.NewObjectValue()` |
| `rt.RunString(src)` | `m.Eval(src)` |
| `rt.Interrupt(reason)` | `m.Interrupt()` / deadline via `m.SetEvalTimeout` |
| `func(goja.FunctionCall) goja.Value` | `RegisterFunc(name, fn any, wantThis bool)` |

The `Keys()` gap is the only structural one: key enumeration must round-trip through JS. It is confined to two call sites (`js_natives.go:140-141`, `js_crypto.go:22`).

### D6: Enforce the invoke budget with `SetEvalTimeout`

Today the JS path has **no** timeout mechanism: `js.go` contains no `Interrupt`, no deadline check, and `curCtx` is only documented as being readable by natives. goja offers `Interrupt`; quickjs offers `SetEvalTimeout` plus `Interrupt`. The swap is therefore also the natural moment to make the JS path honour the invocation budget that `plugin-runtime` already specifies — closing a real gap rather than preserving it.

## Risks / Trade-offs

- **Silent behavioural drift in the JS helpers.** The largest risk: 1,198 lines of surface re-expressed against a different value model. Mitigation: `helper_parity_test.go` must pass **unchanged** — if it needs editing to pass, the port changed behaviour and that edit is the bug. Treat any modification to it as a red flag, not a fix.
- **`Export()` → `Any()` is not always identity.** goja returns Go-typed values with specific shapes (`map[string]any`, `[]any`, `float64`); quickjs may surface different concrete types. The helper tests catch shape drift, but only if they assert types, not just presence.
- **The single JS plugin is a weak canary.** One 253-line ES5 plugin passing does not prove the surface is correct. Mitigation: port the JS test suite faithfully rather than deleting tests that no longer compile.
- **Two-step landing costs an extra review cycle.** The registry refactor could ship together with the swap. Kept separate deliberately: if the swap stalls, the registry is still a net win, and a bisect between the two changes is meaningful.
- **`modernc.org/quickjs` is young** (`go 1.26.0` module, small issue surface). Mitigation: the engine sits entirely behind the interface, so backing it out is a one-file revert if it disappoints.

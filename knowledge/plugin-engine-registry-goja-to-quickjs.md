---
type: Decision
title: Replace goja with quickjs behind an engine registry
description: "Feasibility and cost of replacing goja with modernc.org/quickjs (pure Go, no cgo), and the decision to collapse per-kind engine dispatch into a registry first so an engine swap touches one file instead of four."
tags: [goisekai, pluginmanager, plugins, goja, quickjs, engine, refactor, decision, cgo]
generated: { by: agent/mcp, at: "2026-11-01T02:45:35Z" }
status: stable
---

## Question

Replace `github.com/dop251/goja` with `modernc.org/quickjs` (canonical repo: https://gitlab.com/cznic/quickjs) for the JS plugin runtime.

## Verified facts about modernc.org/quickjs

Checked by cloning the repo and reading it, not from assumption:

- **Pure Go, no cgo.** Zero `import "C"` across 42 `.go` files. The C engine is transpiled with `ccgo`. Its own README states it is "a pure-Go, CGo-free embeddable JavaScript engine (ES2023)".
- **Dependency weight is nearly free for us:** it depends on `modernc.org/libc` + `modernc.org/libquickjs`, and `modernc.org/libc` is **already in `go.sum`**, pulled by `modernc.org/sqlite`.
- **API maps closely** to goja: `NewVM`, `Close`, `Eval`, `Compile`, `NewObjectValue`, `SetPropertyValue`/`GetPropertyValue`, and `RegisterFunc(name string, f any, wantThis bool) error`.
- **Sandboxing is richer than goja's:** `SetEvalTimeout`, `Interrupt`, `SetMemoryLimit`, `SetMaxStackSize`, `SetGCThreshold`.
- **Value conversion differs.** quickjs `Value` has `.Any()`, `.MarshalJSON()`, `.Property(atom)`, `.Call(this, args...)`, `.IsUndefined()`. There is **no `Keys()`** — object key enumeration must go through JS (`Object.keys`) instead.

## Cost in this repo (measured)

| Surface | Size |
|---|---|
| Production files importing goja | 14 |
| `js_*.go` production lines | 1,198 |
| + JS tests (`js_test`, `js_console_test`, `js_html_test`) | 388 |
| JS plugins that must keep working | **1** (`app_data/plugins/mangzio/main.js`, 253 lines) |

The single JS plugin is **pure ES5** — zero `async`/`await`/arrow/`class`/`let`/`const`/template-literals, only `function` + `var`. Host natives it uses: `log.*`, `http.get_body`, `text.json_blob` / `strip_html` / `normalize_status` / `date_to_iso`, `html.match`, `re.exec`. It never touches crypto or browser.

goja methods actually used: `.ToValue()` ×40, `.Set()` ×33, `.Export()` ×28, `.NewObject()` ×5, `.Get()` ×4, `.ToObject()` ×3, `.Keys()` ×3, `.RunString()` ×2, `.Interrupt()` ×1.

## Decision: refactor before swapping

The swap is cheap **only if** the per-kind dispatch is collapsed first. Right now a new or replaced engine touches four call sites, each a repeated `if p.kind == "..."` chain over five kinds:

- `internal/pluginmanager/api.go:28-43` — invoke dispatch (`callLua`/`callJS`/`callGo`/`callYaegi`/`callWasm`)
- `internal/pluginmanager/lifecycle.go` — 5 kind switch points
- `internal/pluginmanager/reload.go:27-36,121` — 5 kind switch points, incl. duplicate teardown logic
- `internal/pluginmanager/manager.go:26-60` — seven per-kind fields on `loadedPlugin` (`lunar`, `goLib`, `goFns`, `js`, `yaegi`, `wasmMod`, `wasmFns`)

Proposed sequence, as **two separate OpenSpec changes**:

1. **`plugin-engine-registry`** — introduce a minimal `engine` interface (`load`, `call`, `close`), a `map[kind]engine` registration table, and collapse `loadedPlugin`'s seven per-kind fields into one handle. Proof: `go test ./internal/pluginmanager/` green, `helper_parity_test.go` still passing, and the 24 lua + 1 js + 1 go + 19 wasm live plugins all still run.
2. **`plugin-js-engine-quickjs`** — swap the JS engine behind that interface. Required ports: `Export()` → `.Any()`, `Keys()` → `Object.keys` via `Eval`, deadline via `SetEvalTimeout`/`Interrupt`. Proof: mangzio still does search + getChapterList + getPageList, `CGO_ENABLED=0 go build` passes, and `go list -deps` shows no `dop251/goja`.

## Rationale for the registry

The repo deliberately hosts several runtimes (see `plugin-runtime-export-matrix`), and every runtime duplicates the same helper families: `*_html`, `*_crypto`, `*_browser`, `*_natives`. `helper_parity_test.go` (14.7K) exists to keep four engines from drifting. Engine #5 currently means a fifth column in that matrix plus a fifth branch in four switch trees. With a registry, adding or replacing an engine is one file.

## Caveat recorded at decision time

The original motivation offered was *performance*. That is weak here: plugin work is I/O-bound (HTTP to the source, HTML parsing, JSON decode), so engine throughput barely moves wall-clock sync time. The user then clarified the repo is a **harness for trying out Go libraries**, so frequent engine swaps are the point — which makes the cheap-to-swap registry the correct goal, not raw speed.

# Related Concepts
- [Plugin runtime export matrix (lua vs js vs yaegi vs go vs wasm)](plugin-runtime-export-matrix.md): That matrix records the per-runtime ABI and host-helper surface; this decision changes how those runtimes are registered and dispatched, and would add quickjs as the JS entry in place of goja.

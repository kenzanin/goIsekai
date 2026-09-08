## Why

goIsekai currently supports four plugin runtimes (WASM, Lua, JS, Go .so), but none let plugin authors write in Go itself without a toolchain. Scriggo (open2b/scriggo) is a Go interpreter that compiles Go source to bytecode and runs it in a sandboxed VM — plugin authors write standard Go, no compilation step, no CGO, no TinyGo. This fills the gap between "quick Lua/JS script" and "full WASM build."

## What Changes

- Add a new plugin kind `"scriggo"` alongside the existing four kinds in the plugin manager
- Plugins are `.go` files (or directories with `main.go`) executed via Scriggo's interpreter
- Same ABI as all other runtimes: `Init`, `SearchManga`, `GetMangaDetails`, `GetChapterList`, `GetPageList`, optional `GetAltTitles`
- Sandbox: no stdlib imports by default; only `fmt` (for debugging) and a custom `hostnet` package (wrapping `http_request`) are exposed
- Scriggo plugins are auto-discovered from the plugins directory (`*.go` files)
- Lazy-load on first ABI call (consistent with existing lazy-load behavior)

## Capabilities

### New Capabilities
- `scriggo-runtime`: Scriggo Go-interpreter plugin execution — loading, sandboxing, ABI dispatch, error handling, and lifecycle for `.go` plugin files

### Modified Capabilities
- `plugin-abi`: Add `"scriggo"` to the plugin kind enum; document that Scriggo plugins implement the same ABI function signatures

## Impact

- `internal/pluginmanager/` — new `scriggo.go` runtime implementation, updated `manager.go` dispatch
- `pkg/types/plugin.go` — add `"scriggo"` to kind constants
- `go.mod` — add `github.com/open2b/scriggo` dependency
- No breaking changes to existing runtimes

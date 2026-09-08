## Context

See proposal.md — Why. Scriggo research (lib-13) confirms: Go interpreter compiles source to bytecode, runs in a register VM, sandboxed by default (no imports unless whitelisted), interruptible via `context.Context`, pure Go with no `unsafe`. Key constraints: pre-1.0 (API churn), no method declarations on script types, no interface type definitions, memory unbounded, types leak (not GC'd).

## Goals / Non-Goals

**Goals:**
- Scriggo as a 5th plugin runtime alongside WASM/Lua/JS/Go
- Same ABI contract as existing runtimes (Init, SearchManga, GetMangaDetails, GetChapterList, GetPageList, optional GetAltTitles)
- Sandboxed execution: no stdlib imports by default, context-based timeout, panic isolation
- Lazy-load on first ABI call (consistent with existing plugin lifecycle)
- Minimal host-side wrapper around `scriggo.Build` + `Program.Run`

**Non-Goals:**
- Replacing Jet templates with Scriggo templates (rejected: syntax incompatible, 70× more allocations, no gain)
- Multi-file Go module support (single `main.go` per plugin is sufficient)
- Method declarations on script-defined types (Scriggo limitation #458)
- Interface type definitions in scripts (Scriggo limitation #218)
- Memory bounding (Scriggo FAQ: not possible; mitigation = context timeout + host process isolation)

## Decisions

### 1. Plugin file layout

**Decision:** One directory per plugin under `plugins/scriggo/<id>/`, containing `main.go`. Auto-discovered by scanning for `main.go` in subdirectories.

**Rationale:** Consistent with Lua plugin layout (`plugins/lua/<id>/main.lua`). Keeps plugins self-contained and version-controllable. Scriggo's `Build` accepts any `fs.FS` — we pass `os.DirFS` rooted at the plugin directory.

**Alternative considered:** Single `.go` file per plugin (like WASM). Rejected — Go modules with `go.mod` + imports are the natural Scriggo pattern; a single file is too restrictive for real plugins.

### 2. Sandboxing via package whitelist

**Decision:** Only `fmt` (for debugging) and a custom `hostnet` package are exposed. Everything else blocked.

**Rationale:** Scriggo's default is zero imports. We expose `fmt` so plugin authors can debug. We expose a `hostnet` package (wrapping the host's `http_request` function) so plugins can make HTTP requests through the TLS-fingerprinted client — same contract as WASM/Lua/JS plugins.

**Alternative considered:** Exposing more stdlib packages. Rejected — the fewer packages available, the smaller the attack surface. Plugin authors who need more can use WASM (full Go stdlib).

### 3. ABI dispatch via exported functions

**Decision:** The host calls Scriggo's `Program.Run` with a small Go "shim" that imports the plugin's package and calls the requested ABI function. The shim is constructed at runtime by string-building a `main.go` that imports the plugin's package.

**Rationale:** Scriggo's `Program.Run` executes `func main()`. To call a specific ABI function, we generate a shim that calls it and marshals the result to JSON. This keeps the plugin's source code clean (no boilerplate `main` function needed).

**Alternative considered:** Requiring plugins to have a `main` function that reads stdin. Rejected — unnatural for Go developers, breaks the "same ABI as other runtimes" contract.

### 4. Timeout via context cancellation

**Decision:** Pass a `context.WithTimeout` to `Program.Run`. Scriggo's VM checks context cancellation at every instruction.

**Rationale:** Scriggo's documented interrupt mechanism. Consistent with WASM's 15s timeout. The context deadline is configurable per plugin (default 15s, same as WASM).

**Alternative considered:** Goroutine kill via `runtime.Goexit`. Rejected — unsafe, can leak resources, not supported by Scriggo.

### 5. Error handling via panic recovery

**Decision:** Wrap `Program.Run` in a `recover()` to catch panics from the interpreted code. Convert panics to errors with the plugin ID and function name.

**Rationale:** Scriggo plugins can panic (nil dereference, index out of range, etc.). The host must not crash. Panic recovery is the same pattern used for WASM (panic isolation) and Lua (protected calls).

### 6. Lazy-load on first ABI call

**Decision:** The plugin manager registers Scriggo plugins at discovery time but does not instantiate the interpreter until the first ABI call. The interpreter is cached per plugin for subsequent calls.

**Rationale:** Consistent with existing lazy-load behavior for all runtimes. Avoids allocating interpreter resources for unused plugins.

**Alternative considered:** Eager instantiation at discovery. Rejected — wastes memory for plugins that are never called.

### 7. Dependency version pinning

**Decision:** Pin `github.com/open2b/scriggo` to `v0.61.1` (latest at time of writing). Update manually after testing.

**Rationale:** Scriggo is pre-1.0 with routine API churn (0.x breaking changes every few releases). Pinning prevents surprise breakage from `go get -u`.

## Risks / Trade-offs

- **[Scriggo pre-1.0 API churn]** → Pin version, test before upgrading, budget for migration work on updates
- **[Memory unbounded — no allocation limits]** → Context timeout kills runaway plugins; host process isolation as fallback; document limitation in plugin author guide
- **[Type definitions leak (not GC'd)]** → Acceptable for long-running plugins (manga sources don't define many types); document in plugin author guide
- **[No method declarations on script types (#458)]** → Plugin authors use package-level functions instead of methods; document in plugin author guide
- **[No interface type definitions (#218)]** → Plugin authors use concrete types; document in plugin author guide
- **[~104 open issues, mostly compiler edge cases]** → Test with real plugin code before shipping; keep WASM as fallback runtime for complex plugins
- **[Performance: register VM slower than native Go]** → Acceptable for I/O-bound manga scraping (network latency dominates); Scriggo's own benchmarks show throughput on par with mainstream interpreted languages

# Scriggo Plugin Runtime — Tasks

> Implementation notes from research (lib-14, verified against v0.61.1):
> - Scriggo module root's main package allows exactly ONE `.go` file; subpackages also one file each. Importable subpackage dirs are fine.
> - `scriggo.Build(fsys, opts)` compiles a whole module; `prog.Run(&RunOptions{Context})` executes `func main` and returns ONLY `error`. No public API exists to call an interpreted function directly.
> - Host↔interpreted data must flow through native packages registered in `BuildOptions.Packages` (declarations: func values or `*var` pointers — pointer vars let interpreted code write back into host variables). `BuildOptions.Globals` is templates-only.
> - Sandbox is default-on: an import resolves only if it names a registered native package. Providing `"fmt"` requires registering it yourself.
> - Timeout: pass a cancelable `context.Context` in `RunOptions.Context`; a `for {}` plugin returns `context.DeadlineExceeded`.
> - Panics in interpreted code return `*scriggo.PanicError` from `Run` — never a host panic. `recover()` around Run is belt-and-braces.
> - Verified module layout used below: `go.mod` (`module X`), root `main.go` (generated shim), plugin source rewritten into a subpackage dir.

## Phase 1: Dependency & host-side scaffolding

- [x] 1.1 Add `github.com/open2b/scriggo v0.61.1` to `go.mod` (`go get github.com/open2b/scriggo@v0.61.1`) and verify `go build ./...` still passes.
- [x] 1.2 Create `internal/pluginmanager/hostnet.go` — a native package that wraps the host's HTTP proxy for Scriggo plugins. Expose at import path `"hostnet"`, package name `hostnet`, with `Get(url string) (string, error)` and `Post(url, body string) (string, error)` implemented host-side by building the standard `{url,method,headers,body}` request JSON and calling `m.proxy.HandleRequest(pluginID, payload)` (same path as WASM/Lua/JS). Register `"hostnet"` plus a minimal `"fmt"` native package (Println/Sprintf/Printf) in the BuildOptions package set. Verify it compiles.

## Phase 2: Runtime core — internal/pluginmanager/scriggo.go

- [x] 2.1 Create `internal/pluginmanager/scriggo.go` with a `scriggoPlugin` struct (`prog *scriggo.Program`, shared host-side in/out variables, plugin id, dir) and a `Call(fnName, argJSON string) (string, error)` method implementing the same ABI contract as other runtimes (JSON-in / JSON-out string; error propagates with plugin ID + function name). Verify it compiles.
- [x] 2.2 Implement `loadScriggo(id, dir)` — read `<dir>/main.go`, rewrite its `package main` clause to `package plugin`, then `scriggo.Build` a virtual `scriggo.Files` module: `go.mod` (`module goisekai.scriggo.<id>`), generated root `main.go` (shim), and `plugin/main.go` (rewritten plugin source). The shim imports the `plugin` package + a host `hostapi` native package, reads `hostapi.Fn` / `hostapi.Arg` (host variables), switches over the ABI functions present in the source (Search, GetMangaDetail, GetChapterList, GetPageList, GetAltTitles if present, Init if present), calls the matched exported function with the arg string, and writes `hostapi.Out` / `hostapi.ErrMsg` back. ABI function names match `pkg/types` constants (e.g. `Search`, `GetMangaDetail`, `GetAltTitles`) and Init returns the PluginMeta JSON string when defined (mirrors WASM `readInit`). Verify a minimal plugin builds.
- [x] 2.3 Wire `hostapi` in BuildOptions.Packages with pointer-backed variables (`Fn`, `Arg` host-written before Run; `Out`, `ErrMsg` interpreted-written, host-read after Run) plus the `hostnet` and `fmt` packages from task 1.2. Guard the shared variables with the per-plugin mutex (one in-flight Run per plugin).
- [x] 2.4 Implement sandboxing — register ONLY `hostnet`, `fmt`, and `hostapi` (no stdlib, no `os`). Verify a plugin containing `import "os"` fails at Build with a clear "package not available" error containing plugin id.
- [x] 2.5 Implement the execution timeout — run `prog.Run` with `RunOptions{Context: ctx}` where `ctx` has the existing `invokeTimeout` deadline (15s); map `context.DeadlineExceeded` to a descriptive error. Verify with an infinite-loop plugin.
- [x] 2.6 Implement panic isolation — treat `*scriggo.PanicError` from `Run` as a plugin error (plugin ID + function name + panic message); wrap Run in `recover()` as a defensive backstop. Verify a panicking plugin returns an error without crashing the host.
- [x] 2.7 Validate `Call` output is well-formed JSON before returning (consistent with `callGo`), and route plugin `Init` metadata (when exported) through the same path so `meta` populates identically to other runtimes.

## Phase 3: Manager integration — internal/pluginmanager/manager.go

- [x] 3.1 Add Scriggo discovery in `Discover()`: scan `pluginsDir/*/main.go` and register each as kind `"scriggo"` (id = parent dir name, `wasmPath` = dir), with the same id-collision skip/log behavior as Lua/JS.
- [x] 3.2 Add the `"scriggo"` case to `ensureLoaded()` (call `loadScriggo`, copy `prog`/vars into the `loadedPlugin`, set `meta`, mark `loaded`, call `proxy.SetNeedsJS`), and add the `scriggo` field to the `loadedPlugin` struct.
- [x] 3.3 Add the `"scriggo"` dispatch in `call()` (api.go), and extend `Install()` / `LoadPlugin()` / `ReloadPlugin()` / `UnloadPlugin()` / `Close()` to handle `main.go` folders the same way Lua/JS folders are handled (copy dir on install; drop program reference on unload/close — Scriggo has no Close, GC reclaims).

## Phase 4: Tests — internal/pluginmanager/scriggo_test.go

- [x] 4.1 Test that a minimal plugin (main.go exporting `Search`) builds, dispatches, and returns a JSON result through `Call`.
- [x] 4.2 Test sandbox violation — plugin with `import "os"` returns a build error naming the plugin.
- [x] 4.3 Test timeout — plugin with an infinite loop returns a context-deadline-exceeded error.
- [x] 4.4 Test panic recovery — a panicking plugin returns an error and the manager still serves the next call.

## Phase 5: Example plugin

- [x] 5.1 Create `examples/plugins/scriggo/<id>/main.go` — a real Go plugin (`package main`, exported ABI funcs) demonstrating Search returning a canned `[]types.Manga`-shaped JSON, a `hostnet.Get` HTTP call, `fmt` usage, and optional Init metadata. Model it on an existing source plugin's ABI behavior.
- [x] 5.2 Install the example into `app_data/plugins/` and verify it appears in the plugins list and answers through the sandbox/API playground routes.

## Phase 6: Verification

- [x] 6.1 Run `go build ./...` and verify no compilation errors.
- [x] 6.2 Run `make check` (fmt, test, vet, lint) and verify green (except pre-existing environment-only failures).
    - Note: 3 hostnet challenge tests were failing on a PRE-EXISTING regression — commit 3bb5be2's dead-code sweep removed `ChallengeError.Unwrap()`, breaking `errors.Is(err, ErrChallenge)`. Restored `Unwrap` in internal/hostnet/verify.go; full suite green.
- [x] 6.3 Restart the server, confirm Scriggo plugins are discovered/listed and callable end-to-end.

# Tasks: lua-plugin-runtime

## 1. Dependencies + plumbing

- [x] 1.1 Add `github.com/yuin/gopher-lua` + `layeh.com/gopher-json` to `go.mod`; `go mod tidy`; verify `CGO_ENABLED=0 go build ./...` stays green
- [x] 1.2 Add `kind` (`"wasm"|"lua"`) + `lua *lua.LState` to `loadedPlugin` in `manager.go`; set `kind="wasm"` on the existing WASM load path

## 2. Lua loader (`internal/pluginmanager/lua.go`)

- [x] 2.1 `loadLua(id, dir string) (*loadedPlugin, error)`: new LState with curated stdlib (`string`, `table`, `math`, `os.time/date/clock` only), rewrite `package.path`/`cpath` to `<dir>/?.lua` + `!<dir>/?.lua`, `SetContext(m.ctx)`
- [x] 2.2 Run `main.lua` via `DoFile`; read `PLUGIN` table → gopher-json → `types.PluginMeta`; `types.CheckVersion` on `contract_version`; error if the four ABI globals are missing/not functions
- [x] 2.3 Register `http_request` global as a Go closure: table→JSON via gopher-json → `proxy.HandleRequest(id, json)` → response JSON→table; on proxy error return `{status=0, error=err}` (no trap)
- [x] 2.4 `callLua(p, fnName, inputJSON)`: `GetGlobal` → push input (raw JSON string; functions may also accept plain strings/tables via gopher-json coercion) → `PCallWithContext` under `invokeTimeout` → normalize result (table→JSON via gopher-json; string passthrough) → return JSON string
- [x] 2.5 `Close` integration: close Lua LStates when the Manager closes

## 3. Discovery + Install

- [x] 3.1 `Discover()`: glob `*/main.lua` alongside `*.wasm`; folder base name = plugin ID; collision with same-ID `.wasm` fails the later load with a descriptive error
- [x] 3.2 `Install(path)`: dir-with-`main.lua` → recursive copy into `app_data/plugins/<id>/`; `.wasm` → existing path; store the `main.lua` path as the plugin's WasmPath
- [x] 3.3 `LoadedPlugins()` unchanged output (verify both kinds appear with meta from `PLUGIN`/Init)

## 4. Example plugin

- [x] 4.1 Create `plugins/lua/kaliscan/` collection skeleton: `main.lua` with `PLUGIN` metadata + four ABI functions using `http_request` and `string.match` parsing helpers in sibling `util.lua` (site reachable → wire real parsing; otherwise leave clearly-marked TODO stubs)
- [x] 4.2 Makefile: `install-lua` target (`make install-lua PLUGIN=kaliscan` copies `plugins/lua/$(PLUGIN)/` into the configured plugins dir)

## 5. Tests

- [x] 5.1 Unit: load a testdata Lua plugin; assert discovery, `PLUGIN` meta parsing, contract-version rejection, missing-global rejection
- [x] 5.2 Unit: `require` sandbox — sibling module loads; `require("../escape")` fails
- [x] 5.3 Unit: `io.open`/`os.execute` unavailable (nil index error)
- [x] 5.4 Unit: `http_request` happy path through a fake proxy handler (assert plugin-ID keying + cookie/UA pass-through shape) and error path (`status=0`)
- [x] 5.5 Unit: infinite-loop Lua function aborts at timeout (use a short override) and surfaces a Go error
- [x] 5.6 Integration: mixed dir (`foo.wasm` + `bar/main.lua`) — both discover, Search via each kind returns decoded results
- [x] 5.7 Full gate: `go vet`, `gofmt`, `go test ./...`, `make build` (CGO-free)

## 6. Docs + ship

- [x] 6.1 README: plugin section gains a "Lua plugins" subsection (folder layout, `PLUGIN` table, `http_request`, `make install-lua`)
- [x] 6.2 Live E2E: install example plugin, restart, verify it appears in Plugins screen, search/detail/chapters/pages all work through the real UI

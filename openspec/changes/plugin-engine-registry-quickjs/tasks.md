# Tasks

## 1. Engine registry

- [ ] 1.1 Add the `engine` interface (`load`, `call`, `close`) plus a `map[string]engine` registry populated at init for `lua`, `js`, `go`, `yaegi`, `wasm`; return a named error for an unregistered kind
- [ ] 1.2 Move each runtime's state and teardown behind its engine implementation, starting with `lua` and `js`, so `loadedPlugin` drops its per-kind typed fields one at a time
- [ ] 1.3 Route `api.go` invoke dispatch through the registry lookup, deleting the five-branch chain
- [ ] 1.4 Route `lifecycle.go` load/unload through the registry, and remove the duplicated `lua` close at `reload.go:121`
- [ ] 1.5 Finish removing the remaining per-kind fields (`goLib`, `goFns`, `wasmMod`, `wasmFns`, `yaegi`) so `loadedPlugin` holds one engine handle
- [ ] 1.6 Add a test that walks every registered kind and asserts load → call → close is reachable through the registry alone, and one that an unknown kind errors without panicking
- [ ] 1.7 Verify registry refactor in isolation: `go test ./internal/pluginmanager/ -count=1`, `helper_parity_test.go` unchanged and passing, `just check`, `just build`, then restart and confirm lua + js + go + wasm plugins all still load and run

## 2. JavaScript engine swap to quickjs

- [ ] 2.1 Swap the module: drop `github.com/dop251/goja`, add `modernc.org/quickjs`; confirm `modernc.org/libc` is already present via sqlite so `go.sum` grows minimally
- [ ] 2.2 Rewrite the VM bootstrap (`js.go`): `NewVM`, ABI export resolution for `search`/`getMangaDetail`/`getChapterList`/`getPageList`/`getGenres`, the legacy-name fallback, and `Close` on unload
- [ ] 2.3 Port the native registration helpers to `RegisterFunc(name, fn any, wantThis bool)` — `js_natives.go`, `js_helpers.go`, `params_native.go`, `js_json.go`, `js_http.go`, `js_console.go`, `js_regex.go`
- [ ] 2.4 Replace `Export()` with `Any()` across the surface and re-assert the concrete Go types the helpers hand back, not just their presence
- [ ] 2.5 Replace the two `Keys()` call sites (`js_natives.go:140-141`, `js_crypto.go:22`) with key enumeration evaluated in the VM
- [ ] 2.6 Port `js_html.go` / `js_html_find.go` / `js_browser.go`, preserving the opaque handle contract and the flat `html` group
- [ ] 2.7 Enforce the invoke budget with `SetEvalTimeout` + `Interrupt`, and assert a non-terminating plugin times out while later calls on that plugin still work
- [ ] 2.8 Port the JS test suite (`js_test.go`, `js_console_test.go`, `js_html_test.go`) rather than deleting cases that no longer compile

## 3. Gate and live verification

- [ ] 3.1 `go test ./internal/pluginmanager/ -count=1` green with `helper_parity_test.go` **unedited**
- [ ] 3.2 `CGO_ENABLED=0 go build ./...` succeeds, and `go list -deps ./internal/pluginmanager` shows no `dop251/goja`
- [ ] 3.3 `just check` → 0 issues, `just build` → binary
- [ ] 3.4 Restart the host and confirm the 24 lua + 1 go + 19 wasm plugins still load and run unchanged
- [ ] 3.5 Run the shipped JS plugin (`mangzio`) end to end on the live server: search, chapter list, page list, and a detail page, comparing shapes against the pre-swap behaviour
- [ ] 3.6 Confirm chapter dates and genres still render on a detail page, so the recent host-side ABI changes did not regress under the new engine

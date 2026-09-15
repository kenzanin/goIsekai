## 1. Shared JSON codec

- [x] 1.1 Add `luaJSONDecode(state)` and `luaJSONEncode(state)` natives in `internal/pluginmanager/lua_json.go`, reusing `lunarToGo` / `goLunarValue` from `lua_convert.go` and `goccy/go-json`; verify with a Lua test that decodes an object and an array and re-encodes both
- [x] 1.2 Add `jsJSONDecode(vm)` and `jsJSONEncode(vm)` in `internal/pluginmanager/js_json.go`, marshalling through `goccy/go-json` and converting with goja's `Export`/`ToValue`; verify with a JS test that decodes an object and an array and re-encodes both
- [x] 1.3 Verify failure reporting in both runtimes: decoding malformed JSON fails, decoding a non-string argument fails, and encoding an unrepresentable value such as a function fails -- each with no partial value returned

## 2. Register the host.json group

- [x] 2.1 Add a `json` group with `decode` and `encode` to `registerHostNatives` in `internal/pluginmanager/lua_natives.go`, beside `text`, `codecs` and `crypto`; verify a Lua fixture reaches `host.json.decode`/`host.json.encode` and that `host.text.strip_html` and `host.codecs.base64_encode` still behave as before
- [x] 2.2 Add the same `json` group to `registerJSHostNatives` in `internal/pluginmanager/js_natives.go`; verify a JS fixture reaches `host.json.decode`/`host.json.encode`
- [x] 2.3 Add a boundary test asserting both runtimes return the identical encoded string for the same value and the identical error text for the same malformed document, so the cross-runtime identity requirement is machine-checked

## 3. Remove the Lua json global

- [x] 3.1 Delete the `json` table registration from `setupGlobals` in `internal/pluginmanager/lua_globals.go`, leaving `log` and `http_request` registered; verify the package builds and the remaining globals still register
- [x] 3.2 Verify a Lua plugin calling `json.encode` or `json.decode` now errors with "attempt to index a nil value" instead of succeeding
- [x] 3.3 Migrate `internal/pluginmanager/testdata/luatest/main.lua` (8 call sites) from `json.decode`/`json.encode` to `host.json.decode`/`host.json.encode`; verify `go test ./internal/pluginmanager/` passes

## 4. Documentation

- [x] 4.1 Update the per-runtime host surface inventory in `docs/host-native-helpers-plan.md` so Lua no longer lists a bare `json` global and both runtimes list `host.json.*`; verify a plugin author reading the removal note can find the replacement call

## 5. Migration and full verification

- [x] 5.1 Produce the Lua plugin migration mapping (`json.decode` -> `host.json.decode`, `json.encode` -> `host.json.encode`) and apply it to the installed Lua plugins under `app_data/plugins/`; verify each migrated plugin loads and its search returns results
- [x] 5.2 Run `CGO_ENABLED=0 go test ./internal/... ./pkg/... ./cmd/...` and verify all tests pass
- [x] 5.3 Run `golangci-lint run ./internal/... ./pkg/... ./cmd/...` and verify no new findings
- [x] 5.4 Exercise a JSON round trip in the running app (search through a migrated Lua plugin and through a JS plugin) and verify the results are unchanged from before the migration

# Tasks

## 1. Rename the kind string

- [x] 1.1 Rename the registry key `"go"` to `"native"` in `internal/pluginmanager/engine.go` (the `engines` map entry), and update `goEngine`'s doc comment if it names the kind; verify with `go test ./internal/pluginmanager/ -run 'TestEngineForResolvesEveryShippedKind|TestEngineForUnknownKindErrors'` passing after 1.4 updates the expectation list
- [x] 1.2 Rename the kind literal at every site that produces or forwards it: `goplug.go` (`pluginMetaFor(id, ...)` and `loadedPlugin.kind`), `goplug_call.go` (`registerFolderPlugin(id, dir, ...)`), and `load.go` (the hot-load log `"kind", ...`); verify with `go build ./...` succeeding
- [x] 1.3 Update the kind comments in `manager.go` and `manager_meta.go` to list `native` instead of `go`, and the `PluginView`/`InfoPluginView` comments in `internal/httpserver/views_plugins.go`; verify with `rg -n '"go"' internal/pluginmanager/*.go internal/httpserver/*.go` returning no kind-literal match
- [x] 1.4 Update the tests that pin the old spelling: the shipped-kind list in `engine_test.go`, the registry fixture key in `engine_registry_test.go`, and the `Kind != "go"` assertion in `goplug_native_test.go`; verify with `go test ./internal/pluginmanager/ -count=1 -run 'TestEngine|TestRegistry|TestNative|TestGo'` passing

## 2. Rename the UI badge key

- [x] 2.1 Rename the `KIND_BADGE` key `go =` to `native =` in both `internal/templates/partials/plugin_card.lua` and `internal/templates/partials/plugin_cards.lua`, keeping the label `Native`; verify with `go test ./internal/templates/ -count=1` passing, including the golden render

## 3. Gate

- [x] 3.1 Run the full gate — `go test ./internal/pluginmanager/ -count=1`, `go test ./internal/templates/ -count=1`, `just check`, `just build` — and confirm every command succeeds; verify by each command exiting 0
- [x] 3.2 Restart the server and confirm a shared-library plugin still loads and shows the native badge: `pkill -x goisekai; sleep 2; (setsid nohup ./goisekai > goisekai.log 2>&1 &); sleep 8; curl -s -o /dev/null -w "%{http_code}" http://127.0.0.1:8080/` returns 200 and `/api/plugins` lists `roliascan` and `madarascans`; verify by the HTTP status and the plugin list

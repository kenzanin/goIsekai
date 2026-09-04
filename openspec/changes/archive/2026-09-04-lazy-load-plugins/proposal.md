## Why

`Discover()` eagerly loads every plugin at startup — compiling WASM modules, instantiating Lua/JS VMs, and calling `contract_version` + `Init` on each. With 3 WASM plugins (~4.5MB each) plus Lua and JS plugins, this adds seconds to startup and holds ~15MB+ of runtime memory even for plugins the user never touches in a session. Deferring runtime instantiation to first use cuts startup time and idle memory.

## What Changes

- `Discover()` scans the plugin directory and registers metadata (id, kind, path, DB record) without instantiating the WASM/Lua/JS runtime
- A new `ensureLoaded(id)` internal method instantiates the runtime on first call to any ABI function (search, detail, chapters, pages)
- All public methods that invoke plugin ABI (`Call`, `CallSearch`, etc.) route through `ensureLoaded` transparently
- `LoadedPlugins()` reports both registered (not-yet-loaded) and loaded plugins, with a `Loaded` boolean
- `UnloadPlugin` releases the runtime and reverts the plugin to registered-only state (can be re-loaded on next call)
- `ReloadPlugin` unloads then re-loads on next call (same as today but without eager re-instantiation)
- `Close()` only closes plugins that have been loaded (skips registered-only)

## Capabilities

### New Capabilities

(none)

### Modified Capabilities

- `plugin-runtime`: Plugin discovery and initialization requirements change from eager-load-at-boot to scan-at-boot + load-on-first-call. New requirement for deferred loading and load-state reporting.

## Impact

- `internal/pluginmanager/manager.go` — `Discover()`, `loadedPlugin` struct, new `ensureLoaded`, `Call` dispatch, `LoadedPlugins`, `Close`
- `internal/pluginmanager/api.go` — call dispatch must route through `ensureLoaded`
- `internal/pluginmanager/lua.go`, `js.go`, `wasm.go` — load functions unchanged but called lazily
- `internal/database/plugins.go` — `RegisterPlugin` still called at scan time (metadata-only)
- `internal/httpserver/views.go`, `plugins.go` — `LoadedPlugins` response shape gains `Loaded` field
- Startup time improvement; no behavioral change to search/detail/chapters/pages

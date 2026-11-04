# Proposal: rename the `.so` plugin kind from `go` to `native`

## Why

The plugin kind string `go` names the language a shared library was written in, not the runtime the host runs it on, and it collides with `yaegi` — the backend that actually interprets Go source. The `.so` backend loads any native shared library through purego `dlopen`, so `go` is wrong twice over: it excludes the C/C++ shared libraries that already ship under the same backend (`examples/plugins/native/`, `app_data/plugins/roliascan`, `app_data/plugins/madarascans`), and it suggests to plugin authors that only Go-compiled artifacts qualify.

`native` names what the runtime actually does and matches the terms the project already uses: the README section "Native plugins (C/C++ .so, no cgo)", the `examples/plugins/native/` directory, and `loadGo`'s own doc comment ("opens a native .so plugin").

## What Changes

- Rename the engine registry key `"go"` to `"native"` in `internal/pluginmanager/engine.go`.
- Rename the kind literal passed through `goplug.go` (`pluginMetaFor`, `loadedPlugin.kind`), `goplug_call.go` (`registerFolderPlugin`) and `load.go` (hot-load log).
- Update the kind comments in `manager.go` and `manager_meta.go` to list `native` instead of `go`.
- Update the `KIND_BADGE` keys in `internal/templates/partials/plugin_card.lua` and `plugin_cards.lua`: `go =` becomes `native =`. The visible label stays `Native`; only the lookup key changes.
- Update the `InfoPluginView`/`PluginView` kind comments in `internal/httpserver/views_plugins.go`.
- No database migration: the `plugins` table has no `kind` column, so the kind exists only in runtime memory.

**Not breaking:** the kind is never persisted and never serialized over HTTP. It reaches the UI only through `KIND_BADGE[p.Kind]`, which this change updates in lockstep, so no alias or compatibility shim is required.

## Capabilities

- **Modified: `plugin-engine-registry`** — the registry's declared kind set changes from `{lua, js, go, yaegi, wasm}` to `{lua, js, native, yaegi, wasm}`.

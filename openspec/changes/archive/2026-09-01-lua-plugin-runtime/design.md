# Design: lua-plugin-runtime

## Context

- Plugin Manager today (`internal/pluginmanager`): WASM-only. `Manager.call()` does the wazero memory dance (malloc → write input JSON → call export → read packed i64 → read output JSON). `hostHTTPRequest()` mirrors it in reverse. `Discover()` globs `*.wasm`; `Install()` copies one `.wasm` file and hot-loads it. Above the Manager (bridge, UI, hostnet, cache, verify) everything is JSON-shaped and runtime-agnostic.
- Deps: `github.com/yuin/gopher-lua` (MIT, pure Go) + `layeh.com/gopher-json` (MIT) for table↔JSON. Both CGO-free — the single-binary `CGO_ENABLED=0` build is preserved.

## Goals / Non-Goals

**Goals:** second plugin kind = folder of `.lua` files with `main.lua` entry; ABI parity with WASM plugins; same hostnet session (fingerprint, cookie jar, verify cookies, UA override); install via recursive folder copy; timeout + per-plugin serialization; `require` locked to the plugin's own folder.

**Non-Goals:** hot-reload on file edit; migrating the mangadex WASM plugin; a Lua debugger; memory capping (accepted risk — Lua has no cap; WASM keeps its 64 MB page limit).

## Decisions

### D1: Plugin kind on `loadedPlugin`, dispatch inside Manager

`loadedPlugin` gains `kind` (`"wasm" | "lua"`) and a `lua *lua.LState` field (nil for WASM). `Manager.call(p, fnName, inputJSON)` dispatches: WASM path unchanged; Lua path = `L.GetGlobal(fnName)` → push input → `L.PCallWithContext` → take result (table or JSON string) → normalize to JSON string. Nothing above `pluginmanager` changes — zero bridge/UI diff.

Plugin ID collision rule: a folder `<id>/main.lua` and a file `<id>.wasm` cannot coexist; Discover fails the load of the later one with a descriptive error (same hard-fail philosophy as contract-version mismatch).

### D2: Entry contract — `PLUGIN` table + global functions

`main.lua` is executed once at load (`L.DoFile`). It must produce:

```lua
PLUGIN = { contract_version = 1, name = "KaliScan",
           verify_url = "", needs_human_verify = false, thumb_ratio = 0.703 }

function search_manga(query, page) ... end        -- -> array of manga tables (or JSON string)
function get_manga_detail(manga_id) ... end       -- -> manga table
function get_chapter_list(manga_id) ... end       -- -> array of chapter tables
function get_page_list(chapter_id) ... end        -- -> array of page tables
```

Load-time checks mirror the WASM loader: read `PLUGIN.contract_version`, `types.CheckVersion`, verify all four globals exist and are functions. `Init metadata` = the `PLUGIN` table (source of verify fields + thumb_ratio for `LoadedPlugins()`), parsed via gopher-json into `types.PluginMeta`. Returning tables OR JSON strings is accepted; gopher-json normalizes tables to JSON, and JSON strings pass through — plugin authors pick whichever is less painful.

### D3: Sandboxing = curated libs + `package.path` lockdown + context

- Register `string`, `table`, `math`, and a reduced `os` (only `time`, `date`, `clock`). No `io`, no full `os`, no `debug`, no `package.loadlib` path.
- `package.path`/`package.cpath` are rewritten to the plugin's own folder only: `<pluginDir>/?.lua`. Relative escapes (`require("../../x")`) resolve outside and fail as module-not-found; the loader never reads outside the folder.
- Timeout: each invocation runs under `L.SetContext(ctx)` with the same 15 s budget (`invokeTimeout`); PCallWithContext aborts at the deadline. Per-plugin `mu` serialization is already enforced by `call()`.

### D4: `http_request` global

Registered once per plugin LState as a Go closure capturing `mod.Name()` (= plugin ID) → `proxy.HandleRequest(pluginID, requestJSON)`. Conversion: Lua table → JSON (gopher-json) → proxy → response JSON → Lua table (gopher-json). Errors return `{status=0, error="..."}` rather than trapping, matching the WASM contract's empty-response-on-failure shape.

### D5: Discovery + Install

- `Discover()`: glob `*.wasm` (existing) plus glob `*/main.lua`; folder base name = ID.
- `Install(path)`: if path is a dir containing `main.lua` → recursive copy into `app_data/plugins/<id>/`; if a `.wasm` file → existing single-file path. `wasm_path` DB column stores the `main.lua` path for Lua plugins (the column is a generic plugin-path in practice; no migration needed).
- Toggle/uninstall/verify flows operate on IDs and never touch plugin files, so they work for both kinds unchanged.

### D6: First-party plugin collection in repo

Source-collection convention: `plugins/lua/<site>/` in the repo (e.g. `plugins/lua/kaliscan/{main.lua,search.lua,util.lua}`). These are sources, not runtime files — the runtime location is always `app_data/plugins/<id>/` after install, same as `.wasm` files built from `plugins/<id>/`. A `Makefile` helper (`make install-lua PLUGIN=kaliscan`) copies a collection folder into the configured plugins dir for local dev.

## Risks / Trade-offs

- **Weaker sandbox than WASM** — a curated-lib Lua VM is safe only as long as no unsafe lib is registered; review `lua.go` registrations before any future lib addition. Accepted for the easy-authoring win.
- **No memory cap** — an allocation-looping Lua plugin can grow host RAM until the 15 s timeout kills it. Accepted (bounded by timeout; WASM remains the hardened tier).
- **gopher-lua maintenance mode** — stable, widely used, pure Go; fine for a scripting tier.

## Migration Plan

None — additive. Existing WASM plugins and the DB schema are untouched.

## Open Questions

- None blocking. (Plugin-screen upload UI for folders can reuse the existing file input with `webkitdirectory` — deferred to implementation; not a contract change.)

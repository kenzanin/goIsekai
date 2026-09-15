# Add Host JSON Helpers

## Why

JSON is the plugin ABI's wire format, yet the two script runtimes hand plugin authors different tools for it. Lua gets a host-backed global `json` table; JS gets nothing from the host and must fall back on the JavaScript VM's built-in `JSON`. Code cannot be shared between Lua and JS plugins, and the `host` helper surface — which exists precisely so shared helpers are written once in Go (`text`, `codecs`, `crypto`, `http`) — has no JSON group at all.

## Changes

- Add a `host.json` helper group with two natives: `host.json.decode(string) -> value` and `host.json.encode(value) -> string`.
- Back both runtimes with the host's `goccy/go-json`, so Lua and JS return the same result and the same error text for the same input, including number formatting, key ordering and failure messages.
- **BREAKING**: remove the Lua global `json` table (`json.encode` / `json.decode`). Lua plugins that use it must switch to `host.json.*`; the global will no longer exist.
- Migrate the in-repo Lua fixture (`internal/pluginmanager/testdata/luatest/main.lua`) to the new surface.
- Document the new group and the removed global alongside the other Lua globals.

## Capabilities

### New Capabilities

- `host-json-helpers`: Host-exposed JSON decode and encode natives available to both the Lua and JS plugin runtimes, with identical semantics and identical error reporting.

### Modified Capabilities

- `plugin-runtime`: the requirement defining the safe Lua stdlib subset currently promises a JSON codec as a global. That global is removed, so the requirement and the exposed-surface contract change.

## Impact

- `internal/pluginmanager/lua_globals.go` — `setupGlobals` no longer registers the `json` table (it keeps `log` and `http_request`).
- `internal/pluginmanager/lua_natives.go` and `js_natives.go` — `registerHostNatives` / `registerJSHostNatives` gain the `json` group beside `text`, `codecs`, `crypto` and `http`.
- `internal/pluginmanager/testdata/luatest/main.lua` — 8 call sites migrate to `host.json`.
- Installed Lua plugins under the gitignored `app_data/plugins/` use the removed global at 199 call sites across 13 files. `Manager.Install` copies a plugin from a local folder, so there is no host-side migration path: those plugins break until their own sources are updated.
- JS plugins keep the VM's built-in `JSON`; `host.json` is the only path guaranteed identical to Lua.
- No database, HTTP API or template change.

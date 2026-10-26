---
description: "A plugin reads its own volatile facts from plugin.json"
id: plugin-json-params
title: A plugin reads its own volatile facts from plugin.json, not from its source
type: Concept
tags: [goisekai, plugins, plugin-json, host-surface, lua, wasm, js, yaegi, native, params]
---

# A plugin reads its own volatile facts from plugin.json

Request: *"nah itu bro kalau sudah di map apa saja yang sering/bisa berubah bisa tidak definisi apa yang sering berubah itu di simpan di plugin.json bro?"*

## Short answer

**Yes - and the host has to do the reading.** A `params` object in `plugin.json` holds the values; the plugin asks for one by name with `host.plugin.param("name")`.

The sandbox cannot read files. Lunar, goja, wazero and yaegi all run with the plugin folder invisible to them, so a plugin that wants its own metadata cannot open `plugin.json` itself. The host reads it at load time and hands the values back.

## The shape

```json
{
  "name": "Atsumaru",
  "site_url": "https://atsu.moe",
  "params": {
    "cdn": "https://cdn.atsu.moe",
    "site_url": "https://atsu.moe"
  }
}
```

```lua
BASE = host.plugin.param("site_url")   -- the site origin
CDN  = host.plugin.param("cdn")        -- a second host, its own fact
```

## The rules that keep it honest

1. **A param the plugin did not declare reads as `""`, not an error.** A plugin that asks for an optional value must still run. This matches the convention the other host helpers already use (empty string = no result).
2. **A param belongs to ONE plugin.** The same helper name must answer differently for two plugins, so `plugin.param` is a `kindRuntime` helper, not a `kindPure` one: it needs the manager and the plugin id, exactly like `http.*` and `browser.*`.
3. **`site_url` is exposed as a param for free.** A plugin that already declares `site_url` gets it without writing it twice. A URL is then written **once** - in `plugin.json` - instead of once in the metadata and once in the source.
4. **The values are ready before the plugin's code runs.** A plugin that builds its URLs at the top level reads them at the top level, so `loadParams` runs before the source is evaluated in all five loaders.
5. **Params are dropped on unload.** They came from a file that may have changed; a reloaded plugin gets the values from its new `plugin.json`.
6. **A param never leaks between plugins.** Two plugins declaring the same name get their own value.

## The host side

| Piece | File | What it does |
| --- | --- | --- |
| `types.PluginParams` | `pkg/types/params.go` | `map[string]string` with a lenient unmarshal |
| `types.PluginMeta.Params` | `pkg/types/types.go` | the `params` field |
| `hostSurface` row | `internal/pluginmanager/host_surface.go` | `{"plugin.param", kindRuntime, ...}` |
| registry | `internal/pluginmanager/params.go` | `setParams` / `pluginParam` / `loadParams` |
| natives | `internal/pluginmanager/params_native.go` | `luaPluginParam`, `jsPluginParam` |

**Why the host holds the values at all:** the sandboxes cannot read the filesystem, so there is no way for the plugin to get them by itself. Injecting them at load is the only route.

**Why the registry has its own lock:** the manager mutex is held while a plugin loads, and plugin code runs inside that window. A lookup that took the manager lock would self-deadlock - `sync.RWMutex` is not reentrant. The registry therefore uses a **leaf** lock, the same pattern `challengeMu` already follows.

**Lenient unmarshal:** a number or a bool in the JSON must not cost the plugin its whole metadata block, because a failed metadata read silently drops the plugin's name and logo. Every value is stringified instead.

## The five runtimes

| Runtime | How a plugin reads a param |
| --- | --- |
| Lua | `host.plugin.param("name")` |
| JS | `host.plugin.param("name")` |
| wasm | `host_call("plugin.param", ["name"])` |
| native `.so` | `host_call("plugin.param", ["name"])` |
| yaegi | `hostnet.Param("name")` |

wasm and native share **one** dispatcher (`dispatchHostCall`), so one case serves both. yaegi gets it as a **bridge method** on `yaegiHostPkg` (like `Get` and `Post`) rather than in the static surface, because the value is per-plugin and the static surface is shared.

The four-way parity tests in `internal/pluginmanager/helper_parity_test.go` are what keep the five from drifting apart.

## The migration that proved it

26 Lua plugins in `examples/plugins/lua/` carried their site facts as literals in the source, duplicated with the `site_url` already in `plugin.json`:

```lua
BASE = "https://atsu.moe"     -- and "site_url": "https://atsu.moe" in plugin.json
```

After the migration every such constant reads a param, and `plugin.json` holds the value:

```lua
BASE = host.plugin.param("site_url")
```

Measured on the whole tree: **26 plugins, 38 constants, 0 URLs lost, 0 gained** (the migration compared the set of URLs in the folder before and after, so a dropped URL would have shown up). `luacheck` on `examples/plugins/lua/` is **byte-identical** before and after (562 warnings, same warning codes) - no new warning, none removed.

Plugins with a **second** host keep it as its own param: `atsumaru` `cdn`, `comizy` `api`, `1manga` `api` + `img_cdn` + `thumb_cdn`, `mangafire` `api` + `referer`, `mangahub` the same three as 1manga, `topmanhua` `cdn`, `valirscans` `media`, `mangadex` `api` + `cdn`.

### The load-order trap

`mangadex/util.lua` is a **sibling module**: `setupRequire` pre-executes every sibling `.lua` before `main.lua`. The host globals were installed *after* that, so a sibling reading `host.plugin.param` at the top level would have hit a nil global. The order is now:

```go
m.loadParams(id, dir)   // the values are known
m.setupGlobals(state, id)  // the host table exists -> a sibling may read a param
setupRequire(state, id, dir) // siblings are pre-executed
```

### The prod-sync trap

`app_data/plugins/` is a **swap area**: a folder there may hold `main.wasm` or `main.so`, and the plugin's own runtime is chosen by what is in the folder. Copying a Lua `main.lua` into a folder that holds a `main.wasm` makes **Lua win discovery** - the log then says `plugin id collision, skipping kind=wasm` and the real runtime never loads. The same copy also stripped `helpers.lua` from a plugin whose `http_get` came from there (`mangafire` answered `attempt to call global 'http_get' (a nil value)`).

**Rule:** sync a plugin's Lua source into `app_data` only into a folder that is a **Lua** folder. A folder holding a `.wasm` or `.so` is not one.

## The tests

`internal/pluginmanager/params_test.go` - a plugin reads a param from its own `plugin.json` (Lua **and** JS), reads it **at load time**, reads `site_url` as a param, reads an undeclared param as `""`, and never sees another plugin's param. Two more cover the shared wasm/native dispatcher and the yaegi `hostnet.Param` bridge.

`internal/pluginmanager/params_declared_test.go` - the guard against the silent failure: **every param a shipped plugin reads must be declared in its `plugin.json`**. An undeclared name reads as `""`, so the plugin keeps running and builds URLs out of an empty string; this test is what makes that visible.

## What was NOT done

- The other runtimes' shipped plugins (wasm, native, yaegi) were not migrated - their sources are not in this tree, and the mechanism is already there for them.
- `UA` (the browser user-agent string) was left in the sources: it is a **host** concern (the host has its own UA and its own TLS fingerprint), not a per-site fact.
- No plugin.json schema validation beyond "the value must be a declared param": a param the plugin never reads is harmless, and a declared param nobody reads is also harmless.

---
type: Concept
title: A plugin reads its own volatile facts from plugin.json
description: "A plugin reads its own volatile facts from plugin.json params via host.plugin.param; host reads at load, values ready before code runs."
tags: [goisekai, plugins, plugin-json, host-surface, lua, wasm, js, yaegi, native, params]
generated: { by: agent/mcp, at: "2026-10-26T15:56:49Z" }
status: stable
---

# A plugin reads its own volatile facts from plugin.json

A plugin reads volatile facts (site URL, CDN, API base) from a `params` object in plugin.json, retrieved via `host.plugin.param(name)`. The sandbox cannot read files, so the host reads plugin.json at load and injects values before the plugin's code runs.

## Shape

```json
{ "name": "Atsumaru", "site_url": "https://atsu.moe", "params": { "cdn": "https://cdn.atsu.moe" } }
```
```lua
CDN = host.plugin.param("cdn")
```

## Rules

1. Undeclared param reads as "" (empty string), not an error.
2. One param belongs to ONE plugin (kindRuntime helper).
3. site_url exposed as a param for free.
4. Values ready before plugin code runs (loadParams before source eval).
5. Params dropped on unload; no leak between plugins.

## Host side

pkg/types/params.go, pkg/types/types.go, internal/pluginmanager/host_surface.go, params.go, params_native.go. Lenient unmarshal (numbers/bools -> string).

## Five runtimes

lua/js: host.plugin.param; wasm/native: host_call("plugin.param", [name]); yaegi: hostnet.Param(name).

## Migration

26 Lua plugins had site facts as literals; after: every constant reads a param. 38 constants -> 0 duplicated URLs. luacheck byte-identical.

## Traps

- Load order: loadParams -> setupGlobals -> setupRequire (mangadex/util.lua pre-executed).
- Prod-sync: sync Lua source only into a Lua folder (Lua wins discovery over wasm).

## Tests

internal/pluginmanager/params_test.go + params_declared_test.go (every param a shipped plugin reads must be declared in plugin.json).

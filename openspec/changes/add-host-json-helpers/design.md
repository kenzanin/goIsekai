# Design: Host JSON Helpers

## Context

See proposal.md — Why.

Current state that constrains the approach:

- The Lua runtime already has a working, host-backed JSON codec. `setupGlobals` in `internal/pluginmanager/lua_globals.go` builds a global `json` table whose `encode` uses `lunarToGo` plus `goccy/go-json` marshal, and whose `decode` uses go-json unmarshal plus `goLunarValue` (both conversions live in `internal/pluginmanager/lua_convert.go`). Nothing new is needed to convert between Lua values and JSON — only to change where it is registered and under what name.
- The shared `host` surface is built in two places: `registerHostNatives` (`lua_natives.go`) and `registerJSHostNatives` (`js_natives.go`). Both build named groups (`text`, `codecs`, `crypto`) and install `host` as a table/object, so a `json` group is a third+1 group in each, not a structural change.
- The existing wrapper helpers (`luaStr1`, `luaStr1Err`, `luaStr2`, `luaStr2Err` in `lua_helpers.go`; the `jsStr*` equivalents in `js_helpers.go`) are all `string`-in/`string`-out. JSON needs a value-shaped signature in both directions, so they cannot be reused as-is.
- JS has no host JSON today; the VM's built-in `JSON` is the only option plugins have.

## Goals / Non-Goals

**Goals:**

- One JSON codec, reachable as `host.json.decode` / `host.json.encode`, present in both script runtimes.
- Byte-for-byte identical results and identical error text between Lua and JS for the same input.
- Reuse the conversions that already ship; do not write a second JSON path.

**Non-Goals:**

- Making `host.json` shadow or remove the JS VM's built-in `JSON`. It stays available; only `host.json` is guaranteed identical to Lua.
- Extending the surface to the Yaegi or Go-plugin runtimes, which already have `encoding/json` and no shared `host` table.
- Any change to the plugin ABI's exported functions (`search_manga`, `get_manga_detail`, ...) — this change only adds a helper callable from plugin code.

## Decisions

**Both runtimes go through the host's `goccy/go-json`, not the JS VM's `JSON`.** The alternative — backing JS with goja's built-in `JSON` — would be less code today but fails the identity requirement: two implementations disagree on number formatting, on `undefined` properties, and above all on error text, so a plugin moved between runtimes would behave differently. One codec removes that class of bug.

**Register under `host.json`, and delete the Lua `json` global.** The alternative was to keep the global as a deprecated alias. It was rejected deliberately: two names for one capability means plugin authors keep discovering the one that is going away, and the shared-surface rationale that already produced `text`/`codecs`/`crypto` argues for exactly one address for each helper. The cost is a breaking change, recorded in the Migration Plan below.

**Add one value-shaped wrapper pair per runtime instead of widening the existing string wrappers.** `luaStr1Err`-style helpers keep their signatures; the JSON natives need `func(string) (any, error)` and `func(any) (string, error)`, so they get their own small constructors next to the existing ones. Widening the shared wrappers would touch every existing helper to serve one new case.

**Keep the decode/encode conversions where they are (`lua_convert.go`).** They are already correct and already exercised by the current global. The move is a registration change, not a rewrite, so the existing behaviour of the codec — including how it distinguishes an array from a map — carries over unchanged.

## Risks / Trade-offs

- Existing Lua plugins break. This is the chosen cost. → Mitigation: the mapping is mechanical (`json.decode` → `host.json.decode`, `json.encode` → `host.json.encode`); the in-repo fixture is migrated in the same change, and the removed global is documented next to the other Lua globals so plugin authors hit an explanation rather than a silent `nil`.
- `host.json` in JS is not the VM's `JSON`. A plugin mixing the two, or one that relies on `JSON.stringify` semantics for values Go cannot marshal (functions, `undefined` members, `Date`), can observe differences. → Mitigation: the identity requirement in the specs is written against `host.json`, and the proposal states that JS's built-in `JSON` remains available for plugins that want ES-specific behaviour.
- Numbers are a lossy round trip on both sides: JSON numbers decoded through Go become `float64`, so integers beyond 2^53 lose precision. → Mitigation: none needed for the wire format the ABI already uses (IDs and titles are strings). Called out here so it is a known limit rather than a surprise.
- An empty Lua table is ambiguous — it can encode as `{}` or `[]` — and the existing codec picks one. Unchanged behaviour, but plugin authors should know. → Mitigation: keep the existing codec rather than inventing a new heuristic mid-change.
- Lua errors surface as a second return value (`nil`, message) while JS errors throw, because that is each runtime's existing convention. The specs require the same *text*, not the same mechanism. → Mitigation: assert error text equality in tests rather than error mechanism.

## Migration Plan

1. Register the `json` group in both `registerHostNatives` and `registerJSHostNatives`; add the value-shaped wrappers.
2. Remove the `json` table registration from `setupGlobals`, leaving `log` and `http_request` in place.
3. Migrate the in-repo Lua fixture and any tracked test that calls the global.
4. Update the exposed-surface documentation to list `host.json` and to state that the Lua `json` global is gone.
5. Out-of-repo work, not part of this change: every installed Lua plugin under `app_data/plugins/` that calls the global must be updated before it will load-and-run again. Because plugins are copied in from local folders, this is the plugin author's step, not something the host can do for them.

Rollback: re-registering the `json` global in `setupGlobals` restores the previous surface without touching the new group, so reverting the removal is a one-function change. Keeping the new `host.json` group does not depend on the global being absent.

## Open Questions

- Whether the Yaegi and Go-plugin runtimes should eventually be given the same `host.json` call for source-level parity. Deferrable: it needs no spec change and no task in this change.

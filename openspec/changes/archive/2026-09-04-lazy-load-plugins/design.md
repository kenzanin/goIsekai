## Context

`Discover()` in `internal/pluginmanager/manager.go` currently loads every plugin at startup: reads the WASM binary, compiles it via Extism, creates Lua/JS VMs, calls `contract_version` and `Init` on each. With 3 WASM plugins (~4.5MB each), 1 Lua plugin, and 1 JS plugin, this adds measurable startup latency and holds runtime memory for plugins the user may never call in a session.

The `loadedPlugin` struct (lines 35-53) holds the compiled runtime (`extismPlugin`, `lua`, or `js`), the resolved `contractVersion`, and the `meta` from Init. The `Manager.plugins` map (`map[string]*loadedPlugin`) is the single source of truth — every ABI call reads from it.

## Goals

1. Startup scans the plugin directory and registers metadata without compiling WASM or creating VMs
2. First ABI call to a plugin transparently instantiates its runtime
3. Subsequent calls reuse the cached runtime (no re-instantiation)
4. `UnloadPlugin` releases runtime resources, reverting to registered-only
5. `LoadedPlugins()` reports load state so the UI can show it
6. No change to the external ABI contract or plugin behavior

## Decisions

### D1: Two-state model — registered vs loaded

The `loadedPlugin` struct gains a `loaded bool` field. A registered-only plugin has `loaded = false` and all runtime fields (`extismPlugin`, `lua`, `js`, `contractVersion`, `meta`) at zero values. `Discover()` creates registered-only entries. `ensureLoaded()` transitions to loaded.

**Why not a separate `registeredPlugin` type?** The `Manager.plugins` map already uses `*loadedPlugin` everywhere. Adding a boolean is a smaller diff and avoids a type-switch at every call site.

### D2: `ensureLoaded(id)` as the single lazy-init gate

A new private method `ensureLoaded(id string) error` checks `p.loaded`; if false, it calls the appropriate `load`/`loadLua`/`loadJS` method (which already exist), sets `loaded = true`, and returns. If true, it's a no-op. Every public ABI method calls `ensureLoaded` before dispatch.

**Concurrency:** `ensureLoaded` acquires `p.mu` (the per-plugin mutex already on `loadedPlugin`) so two goroutines racing to load the same plugin serialize correctly. The first one loads; the second sees `loaded = true` and returns immediately.

### D3: `Discover()` becomes scan-only

`Discover()` iterates the plugin directory, determines kind (wasm/lua/js), creates a `loadedPlugin` with `loaded = false`, registers it in the DB via `RegisterPlugin`, and stores it in `m.plugins`. No `load()`/`loadLua()`/`loadJS()` calls. Error handling: a corrupt file logs a warning and skips (current behavior aborts the whole discovery).

### D4: `UnloadPlugin` releases runtime, keeps registration

`UnloadPlugin(id)` closes the runtime (`extismPlugin.Close()`, `lua.Close()`, or nullifies `js`), sets `loaded = false`, and zeroes the runtime fields. The plugin stays in `m.plugins` and the DB. Next ABI call triggers `ensureLoaded` again.

### D5: `LoadedPlugins()` includes load state

The `LoadedPlugin` struct (exported, line 402) gains `Loaded bool`. `LoadedPlugins()` sets it from `p.loaded`. The plugins template already iterates this list; adding a conditional badge is a one-line template change.

### D6: `Close()` skips registered-only plugins

`Close()` already checks `p.kind` before closing. Adding `if !p.loaded { continue }` avoids touching zero-value runtime fields.

## Risks

- **First-call latency spike:** The first search/detail for a plugin will be slower (WASM compile + Init). Acceptable because it's one-time and user-initiated, not blocking startup.
- **Load-time errors surface later:** A broken plugin that would fail at boot now fails at first use. Mitigated by the existing error envelope and the load-state UI indicator.
- **Concurrency on ensureLoaded:** The per-plugin mutex already serializes ABI calls. Adding `ensureLoaded` inside that mutex scope is safe — no new lock ordering issues.

# Delta: Plugin Runtime — Lazy-load plugins

## MODIFIED Requirements

### Requirement: Plugin discovery
The host SHALL discover plugins by scanning the target plugin directory for `*.wasm` files, `*/main.lua` folders, and `*/main.js` folders. Discovery SHALL register each plugin's metadata (id, kind, path) and persist it to the database without instantiating the WASM/Lua/JS runtime. The plugin SHALL be in a "registered" state — visible in the UI and DB — but its runtime SHALL NOT be allocated until the first ABI call.

#### Scenario: Scan plugin directory at boot
- **WHEN** the host starts
- **THEN** every `.wasm` file, `*/main.lua` folder, and `*/main.js` folder in `app_data/plugins/` is discovered, registered in the DB, and listed in the UI — but no WASM module is compiled and no Lua/JS VM is created

#### Scenario: Discovery failure does not block other plugins
- **WHEN** one plugin file is corrupt or unreadable during scan
- **THEN** the host logs the error and continues registering the remaining plugins

### Requirement: Plugin initialization
The host SHALL defer plugin runtime instantiation (WASM compilation, Lua VM creation, JS VM creation, contract_version check, Init call) until the first ABI function call (search, detail, chapters, pages) for that plugin. The runtime SHALL be instantiated transparently — the caller SHALL NOT need to know whether the plugin is already loaded.

#### Scenario: First search triggers load
- **WHEN** a search request targets a registered-but-not-loaded plugin
- **THEN** the host instantiates the runtime, verifies contract_version, calls Init, caches the loaded instance, and then executes the search — all in one transparent call

#### Scenario: Subsequent calls reuse loaded instance
- **WHEN** a plugin has already been loaded by a prior call
- **THEN** subsequent ABI calls use the cached runtime instance without re-instantiation

#### Scenario: Load failure surfaces as error
- **WHEN** runtime instantiation fails (bad WASM, contract mismatch, Init error)
- **THEN** the host returns the error to the caller and the plugin remains in registered state (retriable on next call)

## ADDED Requirements

### Requirement: Load-state reporting
The host SHALL expose each plugin's load state (registered vs loaded) in the plugin listing. The UI SHALL indicate which plugins have their runtime instantiated and which are registered-only.

#### Scenario: Plugin list shows load state
- **WHEN** the user views the Plugins screen
- **THEN** each plugin card shows whether the plugin runtime is loaded or registered-only (deferred)

### Requirement: Unload releases runtime
The host SHALL allow unloading a plugin's runtime, releasing its WASM/Lua/JS resources and reverting it to registered-only state. The plugin SHALL remain in the DB and UI and SHALL be re-loaded transparently on the next ABI call.

#### Scenario: Unload frees memory
- **WHEN** a loaded plugin is unloaded
- **THEN** its runtime resources (WASM instance, Lua/JS VM) are released and the plugin reverts to registered-only state

#### Scenario: Unloaded plugin re-loads on next call
- **WHEN** an unloaded plugin receives an ABI call
- **THEN** the host re-instantiates the runtime transparently, same as the first load

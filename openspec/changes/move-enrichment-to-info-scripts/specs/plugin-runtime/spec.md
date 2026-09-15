## ADDED Requirements

### Requirement: Enrichment script registration and loading

The host SHALL register every folder containing a `main.lua` under the configured info directory as a Lua runtime, using a namespaced identifier (`info:<folder>`) so a script cannot collide with a manga source plugin whose folder has the same name, while the bare folder name stays the enrichment source id that fetches and the catalog use. Registration SHALL NOT instantiate the runtime: an enrichment script SHALL be instantiated lazily, when a fetch targets it or when the catalog needs the providers it declares. An enrichment script SHALL NOT be held to the manga source ABI — loading SHALL verify `contract_version` and, when the script declares providers, that `getEnrichment` exists, but SHALL NOT require `search_manga`, `get_manga_detail`, `get_chapter_list`, or `get_page_list`. An enrichment script SHALL be excluded from the plugin listing and from load-state reporting.

#### Scenario: Same folder name in both directories
- **WHEN** a manga source plugin folder `mangadex` exists under the plugin directory and an enrichment script folder `mangadex` exists under the info directory
- **THEN** both are registered under distinct ids, the plugin serves searches under `mangadex`, and a metadata fetch naming `mangadex` reaches the script

#### Scenario: Script without the manga source ABI loads
- **WHEN** an info script declares `enrichment_providers` and defines only `getEnrichment`
- **THEN** it loads successfully, while the same file placed in the plugin directory fails to load for missing source globals

#### Scenario: Enrichment script absent from the plugin list
- **WHEN** the plugin list is requested while an enrichment script is registered
- **THEN** no entry for the script appears, and it is not reported as loaded or registered-only

#### Scenario: Unloaded scripts instantiated on demand
- **WHEN** the enrichment catalog is requested and an enrichment script has not been invoked yet
- **THEN** that script is instantiated so its declarations are visible, and no manga source plugin is instantiated

#### Scenario: Id collision inside the info directory
- **WHEN** two folders under the info directory resolve to the same source id
- **THEN** the host logs the collision, skips the later one, and continues registering the remaining scripts

## MODIFIED Requirements

### Requirement: Plugin discovery

The host SHALL discover plugins by scanning the target plugin directory for `*.wasm` files, `*/main.lua` folders, and `*/main.js` folders, and SHALL discover enrichment scripts by scanning the configured info directory for `*/main.lua` folders. Discovery SHALL register each plugin's metadata (id, kind, path) and persist it to the database without instantiating the WASM/Lua/JS runtime. The plugin SHALL be in a "registered" state — visible in the UI and DB — but its runtime SHALL NOT be allocated until the first ABI call. A folder that collides with an already-registered id SHALL be logged and skipped without aborting discovery.

#### Scenario: Scan plugin directory at boot
- **WHEN** the host starts
- **THEN** every `.wasm` file, `*/main.lua` folder, and `*/main.js` folder in `app_data/plugins/` is discovered, registered in the DB, and listed in the UI — but no WASM module is compiled and no Lua/JS VM is created

#### Scenario: Scan info directory at boot
- **WHEN** the host starts with `app_data/info/mangadex/main.lua` present
- **THEN** an enrichment script with source id `mangadex` is registered and no runtime is instantiated for it

#### Scenario: Discovery failure does not block other plugins
- **WHEN** one plugin file is corrupt or unreadable during scan
- **THEN** the host logs the error and continues registering the remaining plugins

#### Scenario: Plugin id collision skipped
- **WHEN** two discovered entries claim the same id
- **THEN** the host logs the collision, skips the later entry, and finishes discovery of the rest

### Requirement: Plugin initialization

The host SHALL defer plugin runtime instantiation (WASM compilation, Lua VM creation, JS VM creation, contract_version check, Init call) until the runtime is first needed: the first ABI function call (search, detail, chapters, pages) for that plugin, the first enrichment fetch for that script, or the catalog read that needs the script's declared providers. The runtime SHALL be instantiated transparently — the caller SHALL NOT need to know whether the plugin is already loaded.

#### Scenario: First search triggers load
- **WHEN** a search request targets a registered-but-not-loaded plugin
- **THEN** the host instantiates the runtime, verifies contract_version, calls Init, caches the loaded instance, and then executes the search — all in one transparent call

#### Scenario: Subsequent calls reuse loaded instance
- **WHEN** a plugin has already been loaded by a prior call
- **THEN** subsequent ABI calls use the cached runtime instance without re-instantiation

#### Scenario: Load failure surfaces as error
- **WHEN** runtime instantiation fails (bad WASM, contract mismatch, Init error)
- **THEN** the host returns the error to the caller and the plugin remains in registered state (retriable on next call)

#### Scenario: Enrichment script load failure is isolated
- **WHEN** an enrichment script fails to load while the enrichment catalog is being read
- **THEN** the host logs the failure, omits that script's sources from the catalog, and returns the remaining sources without error

### Requirement: Load-state reporting

The host SHALL expose each plugin's load state (registered vs loaded) in the plugin listing, for manga source plugins. Enrichment scripts SHALL NOT appear in that listing or its load-state reporting. The UI SHALL indicate which plugins have their runtime instantiated and which are registered-only.

#### Scenario: Plugin list shows load state
- **WHEN** the user views the Plugins screen
- **THEN** each plugin card shows whether the plugin runtime is loaded or registered-only (deferred)

#### Scenario: Enrichment scripts omitted from load state
- **WHEN** the plugin listing is read while enrichment scripts are registered and loaded
- **THEN** no enrichment script appears in it and their load state is not reported

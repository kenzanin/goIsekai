## Delta: Plugin Runtime — Lua plugin kind

Extends the existing plugin-runtime capability with Lua plugins as a second kind alongside WASM. All host-facing behavior above the Manager (bridge, UI, cache, verify) treats both kinds identically.

## ADDED Requirements

### Requirement: Lua plugin discovery via main.lua folder
The host SHALL discover Lua plugins by scanning the plugin directory for folders containing a `main.lua` (`app_data/plugins/*/main.lua`), alongside the existing `*.wasm` discovery. The folder name SHALL be the plugin ID.

#### Scenario: Folder plugin discovered
- **WHEN** the host starts and `app_data/plugins/kaliscan/main.lua` exists
- **THEN** a plugin with ID `kaliscan` is registered and its ABI functions become callable

#### Scenario: Mixed-kind coexistence
- **WHEN** the plugin directory contains both `mangadex.wasm` and `kaliscan/main.lua`
- **THEN** both plugins are discovered, registered, and indistinguishable in the UI

### Requirement: Lua require restricted to plugin folder
The Lua runtime SHALL restrict `require` (via `package.path`) to the plugin's own folder, so a plugin can split into sibling `.lua` modules but cannot load files outside its folder.

#### Scenario: Sibling module loaded
- **WHEN** `main.lua` calls `require("search")` and `search.lua` exists in the same plugin folder
- **THEN** the module loads and its functions are usable

#### Scenario: Escape attempt rejected
- **WHEN** a Lua plugin calls `require("../../other/file")` or requires a path resolving outside its folder
- **THEN** the require fails with an error instead of loading external files

### Requirement: Safe Lua stdlib subset
The host SHALL expose only the safe Lua standard subsets (`string`, `table`, `math`, and `os.time`/`os.date`, `os.clock`) plus a JSON codec; file, IO, process, and unrestricted `os` access SHALL NOT be registered.

#### Scenario: Unsafe library unavailable
- **WHEN** a Lua plugin calls `io.open` or `os.execute`
- **THEN** the call errors with "attempt to index a nil value" (library not registered) rather than touching the host filesystem or processes

### Requirement: Host http_request as Lua global
The host SHALL expose `http_request(request_table) -> response_table` as a Lua global returning `{status, headers, body}`, routed through the same per-plugin hostnet client (TLS fingerprint, cookie jar, verify cookies, User-Agent override) used by WASM plugins.

#### Scenario: Request carries plugin session
- **WHEN** a Lua plugin calls `http_request({url = "https://example.com"})` and the plugin has verify cookies stored
- **THEN** the outgoing request carries those cookies and the plugin's User-Agent override, exactly as a WASM plugin's request would

### Requirement: Lua ABI parity
A Lua plugin SHALL implement the same JSON-in/JSON-out ABI as WASM plugins: a `PLUGIN` metadata table with `contract_version`, optional `verify_url`/`needs_human_verify`/`thumb_ratio`, and global functions `search_manga`, `get_manga_detail`, `get_chapter_list`, `get_page_list`. Tables and JSON strings SHALL be converted transparently at the boundary.

#### Scenario: Init metadata honored
- **WHEN** a Lua plugin's `PLUGIN` table declares `needs_human_verify = true` and a `verify_url`
- **THEN** the Plugins screen shows the Human Verification panel for it, same as a WASM plugin

#### Scenario: Search returns results
- **WHEN** `search_manga(query, page)` returns a Lua array of `{id, title, cover_url}` tables
- **THEN** the host converts it to the same manga result shape a WASM plugin returns and search works in the UI

### Requirement: Lua invocation timeout and serialization
The host SHALL enforce the per-invocation timeout on Lua calls (via interpreter context) and serialize invocations per plugin instance, so a hung or looping plugin cannot block the host indefinitely.

#### Scenario: Infinite loop aborted
- **WHEN** a Lua plugin function loops forever
- **THEN** the invocation is aborted at the timeout and surfaces as a Go error, like a hung WASM plugin

### Requirement: Lua plugin install via folder copy
The Install action SHALL accept a Lua plugin folder and copy it recursively into `app_data/plugins/<id>/` (entry `main.lua`), then hot-load it — mirroring the WASM single-file install.

#### Scenario: Folder installed
- **WHEN** the user installs a Lua plugin folder through the Plugins screen
- **THEN** the folder (with all sibling `.lua` modules) is copied into `app_data/plugins/<id>/` and the plugin is usable immediately

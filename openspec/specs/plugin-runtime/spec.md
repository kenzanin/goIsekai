# Plugin Runtime Specification

## Purpose

Manages the lifecycle of WASM and Lua source plugins — discovery, loading, per-instance resource limits, per-call timeouts, and panic isolation — so a misbehaving plugin never crashes the host or the UI. The runtime also provides host-side human-verification support for challenge-protected sites, converts cached images to WebP, and honors plugin-declared thumbnail aspect ratios.

## Requirements

### Requirement: Plugin discovery
The host SHALL discover plugins by scanning the target plugin directory (`app_data/plugins/*.wasm`).

#### Scenario: Scan plugin directory
- **WHEN** the host starts or the plugin directory is refreshed
- **THEN** every `.wasm` file in `app_data/plugins/` is discovered and made available

### Requirement: Plugin initialization
The host SHALL load each discovered plugin module into the WASM runtime context so its exported host functions become callable.

#### Scenario: Load a valid plugin
- **WHEN** a valid `.wasm` plugin is discovered
- **THEN** the host loads it into the runtime and exposes its functions without starting network access directly

### Requirement: Memory resource limits
The host SHALL enforce a maximum memory allocation per plugin instance (e.g. 64 MB).

#### Scenario: Plugin exceeds memory limit
- **WHEN** a plugin instance attempts to allocate more than the configured memory cap
- **THEN** the host fails the call with an error rather than exhausting host memory

### Requirement: Invocation timeout
The host SHALL enforce a maximum execution time per invocation (e.g. 15 seconds).

#### Scenario: Plugin hangs
- **WHEN** a plugin invocation runs longer than the configured timeout
- **THEN** the host terminates the call and returns a timeout error

### Requirement: Panic isolation
The host SHALL intercept WASM panics and OOM failures and convert them into Go errors without crashing the host process.

#### Scenario: Plugin panics
- **WHEN** a `.wasm` plugin panics or is OOM-killed during a call
- **THEN** the host returns a Go `error` for that call
- **AND** the UI remains fully functional

### Requirement: Human verification session capture
The host SHALL store user-pasted verification cookies and an optional browser User-Agent per plugin id. When present, the host SHALL inject those cookies (and override the User-Agent) into the plugin's HTTP client before every request. Storage SHALL be a `plugin_verify` table keyed by plugin id.

#### Scenario: Paste cookies after challenge
- **WHEN** a plugin is flagged `needs_human_verify` and the user pastes cookies in the Plugins screen verification panel
- **THEN** subsequent host HTTP requests for that plugin carry the pasted cookies and User-Agent

#### Scenario: Tolerant cookie parsing
- **WHEN** the paste is a full `Cookie:` header, a single `name=value` pair, or a bare value
- **THEN** the host parses it into cookie pairs and stores them without error

### Requirement: Challenge detection surfaces verification banner
The host SHALL detect challenge responses (HTTP 403 with challenge markers such as `cf-challenge` or "Just a moment") and the affected view (search/detail) SHALL render a banner instructing the user to verify via the Plugins screen.

#### Scenario: Search hits a challenge
- **WHEN** a search request to a protected site returns a challenge response
- **THEN** the search view renders a "Site needs human verification" banner instead of silently showing no results

### Requirement: Image cache stored as WebP
The host SHALL convert cached images to WebP (quality ~85) at disk-cache write time, except animated or already-WebP images. Served responses SHALL carry the correct `Content-Type`. Existing cache entries SHALL NOT be migrated or invalidated.

#### Scenario: JPEG page cached as WebP
- **WHEN** a JPEG page image is written to the disk cache
- **THEN** it is stored as a `.webp` file at reduced size and served with `Content-Type: image/webp`

### Requirement: Plugin-declared thumbnail aspect ratio
Plugins SHALL be able to declare an optional `thumb_ratio` (width/height) in their Init response. The host SHALL persist it and search/library cards SHALL use it via `aspect-ratio` styling. Plugins without the field SHALL fall back to 2:3.

#### Scenario: MangaDex covers uncropped
- **WHEN** the mangadex plugin declares `thumb_ratio` 0.703
- **THEN** search and library cards render covers at that ratio without cropping

### Requirement: Verification metadata in plugin ABI
The plugin Init response SHALL support optional `verify_url` and `needs_human_verify` fields, persisted at registration, exposed to the Plugins screen.

#### Scenario: Plugin declares verification need
- **WHEN** a plugin's Init response contains `needs_human_verify: true` and a `verify_url`
- **THEN** the Plugins screen shows a Human Verification panel with an "Open verification page" link targeting that URL

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

### Requirement: needs_js plugin hint
The plugin Init response SHALL support an optional `needs_js` boolean. When true, the host SHALL route the plugin's requests through the browser engine rather than the tls-client fast path, provided an engine is configured.

#### Scenario: Plugin declares needs_js
- **WHEN** a plugin's Init response sets `needs_js: true` and an engine is configured
- **THEN** the host uses the browser engine for that plugin's requests instead of the fast path

#### Scenario: needs_js but engine off
- **WHEN** a plugin sets `needs_js: true` but the CDP engine is `off`
- **THEN** the host falls back to the fast path and may surface challenge errors as today

#### Scenario: Hint absent
- **WHEN** a plugin omits `needs_js`
- **THEN** the host treats it as `false` and uses the fast path, with challenge-triggered fallback still available

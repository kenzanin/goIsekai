## Purpose

Manages the lifecycle of WASM source plugins — discovery, loading, per-instance resource limits, per-call timeouts, and panic isolation — so a misbehaving plugin never crashes the host or the UI.

## ADDED Requirements

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
The host SHALL intercept WASM panics and OOM failures and convert them into Go errors without crashing the main Wails host process.

#### Scenario: Plugin panics
- **WHEN** a `.wasm` plugin panics or is OOM-killed during a call
- **THEN** the host returns a Go `error` for that call
- **AND** the UI remains fully functional

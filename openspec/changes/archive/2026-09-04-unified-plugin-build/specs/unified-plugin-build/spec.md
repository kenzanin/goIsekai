## Purpose

Provide a single-command build for all WASM plugins that integrates with the host build, preventing stale plugin binaries.

## ADDED Requirements

### Requirement: Unified plugin build target
A `make build-plugins` target at the repo root SHALL build all WASM plugins by invoking each plugin's Makefile. The target SHALL exit non-zero if any plugin build fails.

#### Scenario: Build all plugins
- **WHEN** the user runs `make build-plugins`
- **THEN** all WASM plugins (mangadex, mangafire, dummy) are rebuilt and their .wasm files are updated

#### Scenario: One plugin fails
- **WHEN** one plugin's build fails
- **THEN** `make build-plugins` exits non-zero and reports which plugin failed

### Requirement: Combined build target
A `make all` target SHALL build all plugins and then build the host binary, in that order.

#### Scenario: Full build
- **WHEN** the user runs `make all`
- **THEN** all WASM plugins are rebuilt, then the host binary is rebuilt

### Requirement: Plugin install target
A `make install-plugins` target SHALL copy built .wasm files from each plugin's `dist/` directory to `app_data/plugins/`.

#### Scenario: Install plugins
- **WHEN** the user runs `make install-plugins`
- **THEN** all .wasm files are copied to app_data/plugins/, overwriting existing files

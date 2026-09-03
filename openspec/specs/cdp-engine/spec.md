# CDP Engine Specification

## Purpose

Provides a host-managed browser engine that solves anti-bot challenges and harvests session cookies, so protected manga sources can be accessed automatically instead of requiring a manual cookie paste.

## Requirements

### Requirement: Engine selection from settings
The host SHALL select the CDP engine from user settings, not from plugin declarations. The setting SHALL support `off` (disabled), `lightpanda`, and `chrome`, with `off` as the default so no browser is used unless the user opts in.

#### Scenario: Engine disabled by default
- **WHEN** the user has not configured `cdp_engine` (or set it to `off`)
- **THEN** the host performs no browser-based solving and surfaces challenge errors as today

#### Scenario: User selects Chrome
- **WHEN** the user sets `cdp_engine=chrome` and provides a valid `cdp_path`
- **THEN** the host drives Chrome via CDP to solve challenges

### Requirement: Browser binary located via setting
The host SHALL locate the browser binary at the path given by the `cdp_path` setting, and SHALL NOT download or bundle a browser binary itself.

#### Scenario: Missing binary
- **WHEN** the engine is enabled but the binary at `cdp_path` does not exist or cannot launch
- **THEN** the host reports an error and falls back to surfacing the challenge (no silent failure)

### Requirement: Challenge solving and cookie harvest
The host SHALL navigate the browser to the challenge URL, wait for the challenge to resolve, and extract the resulting cookies so they can be injected into the plugin's session.

#### Scenario: Challenge solved
- **WHEN** a protected page loads and its anti-bot challenge completes in the browser
- **THEN** the host extracts the session cookies (including `cf_clearance`) from the browser's cookie jar

#### Scenario: Challenge times out
- **WHEN** the browser fails to solve the challenge within a configured timeout
- **THEN** the host aborts the solve, releases the browser, and surfaces the original challenge error

### Requirement: Browser lifecycle isolation
The host SHALL manage the browser as a separate process, and SHALL bound each solve attempt with a timeout so a hung browser cannot block the host indefinitely.

#### Scenario: Browser killed after solve
- **WHEN** a solve completes or times out
- **THEN** the host closes the browser process (or returns it to a reusable pool) so no stray process accumulates

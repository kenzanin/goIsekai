## Purpose

Persist application settings to a config file so they survive restarts, with CLI flags taking precedence over file values.

## ADDED Requirements

### Requirement: Config file read on startup
The host SHALL read `goisekai.ini` from the data directory on startup if it exists. CLI flags SHALL override file values; file values SHALL override defaults. Precedence: flag > file > default.

#### Scenario: File exists with values
- **WHEN** `goisekai.ini` contains `port = 9090` and no `--port` flag is provided
- **THEN** the server listens on port 9090

#### Scenario: Flag overrides file
- **WHEN** `goisekai.ini` contains `port = 9090` and `--port 3000` is passed
- **THEN** the server listens on port 3000

#### Scenario: No file exists
- **WHEN** no `goisekai.ini` exists in the data directory
- **THEN** the host uses defaults and CLI flags as before

### Requirement: Settings page saves config
The Settings page "Save" action SHALL write the current settings to `goisekai.ini` in INI format. The saved values SHALL be read on next startup.

#### Scenario: Save persists settings
- **WHEN** the user changes the API key on the Settings page and clicks Save
- **THEN** `goisekai.ini` is written with the new API key value

#### Scenario: Saved values survive restart
- **WHEN** the server restarts after a settings save
- **THEN** the saved values are loaded from `goisekai.ini` and applied

### Requirement: Hot-reload for safe settings
The host SHALL hot-reload a safe subset of settings (log level, user-agent, referer) when the config file changes, without requiring a restart. Unsafe settings (port, host, CDP engine) SHALL require a restart.

#### Scenario: Log level hot-reload
- **WHEN** the user changes log level in settings and saves
- **THEN** the new log level takes effect immediately without restart

#### Scenario: Port requires restart
- **WHEN** the user changes the port in settings and saves
- **THEN** the new port does not take effect until the server is restarted

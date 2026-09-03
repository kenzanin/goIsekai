## Why
Settings (CDP engine, API key, port, host, log level, user-agent, referer) are CLI-flag-only. Restart loses all configuration. The `goisekai.ini` struct exists in config.go but has no read/write logic.

## What Changes
- Read `goisekai.ini` on startup if it exists
- CLI flags override file values (flag > file > default precedence)
- Settings page "Save" writes config file
- Hot-reload for safe subset (log level, user-agent) without restart

## Capabilities
### New Capabilities
- `config-persistence`: Read/write goisekai.ini config file with CLI-flag override precedence and settings-page save

### Modified Capabilities
(none)

## Impact
- `internal/config/config.go` — add INI read/write logic
- `internal/httpserver/actions.go` — save-settings handler writes config file
- `cmd/goisekai/main.go` — load config file before flag parsing
- No API or plugin ABI changes

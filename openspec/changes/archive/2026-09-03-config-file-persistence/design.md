## Context
`internal/config/config.go` defines a `Config` struct populated entirely by CLI flags. The `goisekai.ini` struct exists but has no read/write logic. Settings are lost on restart.

## Goals
1. Read `goisekai.ini` on startup with flag > file > default precedence
2. Settings page "Save" writes current config to file
3. Hot-reload safe subset (log level, user-agent) without restart

## Decisions

### D1: INI format using go-ini or stdlib
Use `gopkg.in/ini.v1` (already in ecosystem) or hand-roll a simple INI reader/writer. The existing `goisekai.ini` struct suggests INI was the intended format.

### D2: Config load order in main.go
1. Load defaults into Config struct
2. Read goisekai.ini if exists, overlay onto Config
3. Parse CLI flags, overlay onto Config (highest priority)

### D3: Hot-reload via fsnotify or polling
Use `fsnotify` to watch the config file. On change, re-read and apply safe fields only (log level, user-agent, referer). Unsafe fields (port, host, CDP engine) are ignored until restart.

### D4: Settings save handler
The existing `handleSaveSettings` action handler writes to the config file after updating in-memory settings. Uses the same INI writer.

## Risks
- Config file corruption on concurrent writes — mitigated by single-writer (settings page only)
- fsnotify adds a dependency — could use polling instead (check mtime every 5s)

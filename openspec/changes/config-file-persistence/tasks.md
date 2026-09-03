## Tasks

### 1. Add INI read/write to config package
- [ ] 1.1 Add `LoadConfig(path string) (*Config, error)` to `internal/config/config.go` — reads INI file, overlays onto defaults
- [ ] 1.2 Add `SaveConfig(path string, cfg *Config) error` — writes current config to INI file
- [ ] 1.3 Add precedence logic: defaults → file → CLI flags in `cmd/goisekai/main.go`

### 2. Wire settings save to config file
- [ ] 2.1 Update `handleSaveSettings` in `internal/httpserver/actions.go` to call `SaveConfig` after updating in-memory settings
- [ ] 2.2 Ensure config file path is accessible from the action handler (via AppService or config reference)

### 3. Hot-reload safe settings
- [ ] 3.1 Add config file watcher (fsnotify or polling) that re-reads on change
- [ ] 3.2 Apply safe subset (log level, user-agent, referer) immediately; ignore unsafe fields

### 4. Verify
- [ ] 4.1 `go build ./... && go vet ./...` passes
- [ ] 4.2 `go test ./internal/config/` passes
- [ ] 4.3 Live smoke: save settings, restart, verify values persist

## Tasks

### 1. Refactor `loadedPlugin` struct for two-state model
- [x] 1.1 Add `loaded bool` field to `loadedPlugin` struct in `internal/pluginmanager/manager.go`
- [x] 1.2 Add `Loaded bool` field to exported `LoadedPlugin` struct

### 2. Implement `ensureLoaded` lazy-init gate
- [x] 2.1 Add `ensureLoaded(id string) error` method to `Manager` — checks `p.loaded`, calls appropriate `load`/`loadLua`/`loadJS`, sets `loaded = true`, acquires `p.mu` for concurrency safety
- [x] 2.2 Wire `ensureLoaded` into all ABI dispatch paths: `Call` in `api.go` (or wherever search/detail/chapters/pages are dispatched)

### 3. Convert `Discover()` to scan-only
- [x] 3.1 Rewrite `Discover()` to create `loadedPlugin` entries with `loaded = false` — scan directory, determine kind, register in DB, store in `m.plugins`, but do NOT call `load`/`loadLua`/`loadJS`
- [x] 3.2 Change error handling: log-and-skip corrupt plugins instead of aborting the whole discovery

### 4. Update `UnloadPlugin` for registered-only revert
- [x] 4.1 Modify `UnloadPlugin` to close runtime, set `loaded = false`, zero runtime fields — plugin stays in `m.plugins` and DB

### 5. Update `LoadedPlugins` and `Close`
- [x] 5.1 Set `Loaded` field in `LoadedPlugins()` from `p.loaded`
- [x] 5.2 Guard `Close()` with `if !p.loaded { continue }` to skip registered-only plugins

### 6. Update UI for load-state display
- [x] 6.1 Add load-state badge to plugins template (`internal/templates/views/plugins.jet`) — show "Loaded" or "Deferred" per plugin card

### 7. Verify
- [x] 7.1 `go build ./... && go vet ./...` passes
- [x] 7.2 `go test ./internal/pluginmanager/ ./internal/bridge/ ./internal/database/` passes
- [x] 7.3 Live smoke: start server, verify plugins list shows "Deferred" for all, trigger a search, verify plugin transitions to "Loaded"

## Tasks

### 1. Signal handling
- [ ] 1.1 Add `signal.NotifyContext` for SIGTERM/SIGINT in `cmd/goisekai/main.go`
- [ ] 1.2 Pass cancellable context to server, DB, and plugin manager

### 2. HTTP server graceful shutdown
- [ ] 2.1 Add `Shutdown(ctx)` method to `internal/httpserver/Server`
- [ ] 2.2 Wire shutdown to context cancellation with 10s timeout

### 3. Resource cleanup
- [ ] 3.1 Ensure `database.Close()` exists and is called on shutdown
- [ ] 3.2 Call `pluginManager.Close()` on shutdown (already exists)
- [ ] 3.3 Flush log buffer on shutdown

### 4. PID file
- [ ] 4.1 Write PID file on startup in data directory
- [ ] 4.2 Remove PID file on shutdown (defer)
- [ ] 4.3 Add `goisekai stop` subcommand that reads PID and sends SIGTERM

### 5. Verify
- [ ] 5.1 `go build ./... && go vet ./...` passes
- [ ] 5.2 Live smoke: start server, send SIGTERM, verify clean shutdown log
- [ ] 5.3 Verify PID file created on start, removed on stop

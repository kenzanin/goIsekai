## Tasks

### 1. Signal handling
- [x] 1.1 Add `signal.NotifyContext` for SIGTERM/SIGINT in `cmd/goisekai/main.go`
- [x] 1.2 Pass cancellable context to server, DB, and plugin manager

### 2. HTTP server graceful shutdown
- [x] 2.1 Add `Shutdown(ctx)` method to `internal/httpserver/Server`
- [x] 2.2 Wire shutdown to context cancellation with 10s timeout

### 3. Resource cleanup
- [x] 3.1 Ensure `database.Close()` exists and is called on shutdown
- [x] 3.2 Call `pluginManager.Close()` on shutdown (already exists)
- [x] 3.3 Flush log buffer on shutdown

### 4. PID file
- [x] 4.1 Write PID file on startup in data directory
- [x] 4.2 Remove PID file on shutdown (defer)
- [x] 4.3 Add `goisekai stop` subcommand that reads PID and sends SIGTERM

### 5. Verify
- [x] 5.1 `go build ./... && go vet ./...` passes
- [x] 5.2 Live smoke: start server, send SIGTERM, verify clean shutdown log
- [x] 5.3 Verify PID file created on start, removed on stop

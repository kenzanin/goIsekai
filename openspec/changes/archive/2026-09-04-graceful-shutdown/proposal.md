## Why
Server is killed via `pkill`. No signal handler for SIGTERM/SIGINT to drain in-flight requests, close DB connections, flush logs, and stop plugin runtimes cleanly. This risks data corruption and orphaned processes.

## What Changes
- `signal.NotifyContext` on SIGTERM/SIGINT
- `http.Server.Shutdown(ctx)` with timeout
- DB close, plugin manager close, log flush
- PID file for clean stop (`goisekai stop`)

## Capabilities
### New Capabilities
- `graceful-shutdown`: Signal-driven graceful shutdown with request draining and resource cleanup

### Modified Capabilities
(none)

## Impact
- `cmd/goisekai/main.go` — signal handler + shutdown orchestration
- `internal/httpserver/server.go` — expose Shutdown method
- `internal/database/` — expose Close method
- `internal/pluginmanager/` — Close already exists
- No API or plugin ABI changes

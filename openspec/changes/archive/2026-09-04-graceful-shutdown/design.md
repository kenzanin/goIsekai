## Context
The server is started via `go run` or the built binary and killed via `pkill`. No signal handling exists. DB connections, plugin runtimes, and log buffers are abandoned on kill.

## Goals
1. Graceful shutdown on SIGTERM/SIGINT
2. Drain in-flight requests before closing
3. Clean resource teardown (DB, plugins, logs)
4. PID file for management scripts

## Decisions

### D1: Shutdown orchestration in main.go
`main()` creates a cancellable context via `signal.NotifyContext`. The HTTP server, DB, and plugin manager receive this context. On signal, context is cancelled, triggering shutdown cascade.

### D2: HTTP server shutdown
Use `http.Server.Shutdown(ctx)` with a 10s timeout context. This drains in-flight requests before returning.

### D3: Resource cleanup order
1. Stop accepting new HTTP connections (Shutdown)
2. Close plugin manager (stops WASM/Lua/JS runtimes)
3. Close DB connection pool
4. Flush log buffer
5. Remove PID file

### D4: PID file
Write PID on startup, remove on shutdown. `goisekai stop` subcommand reads PID and sends SIGTERM. Simple file-based approach, no daemon complexity.

## Risks
- Long-running plugin calls may exceed shutdown timeout — acceptable, force-kill after timeout
- PID file stale if server crashes without cleanup — `goisekai stop` should check if PID is alive

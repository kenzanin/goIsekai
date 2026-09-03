## Why
No request logging middleware exists. Cannot diagnose slow requests, error rates, or traffic patterns. Debugging requires manual browser devtools or external proxies.

## What Changes
- Middleware: method, path, status, latency, size
- Structured logging (slog) integration
- Skip health-check and static asset noise
- Optional request ID for tracing

## Capabilities
### New Capabilities
- `http-access-logging`: Structured HTTP request logging middleware with noise filtering

### Modified Capabilities
(none)

## Impact
- `internal/httpserver/middleware.go` — new logging middleware
- `internal/httpserver/routes.go` — wire middleware into router
- No API or plugin ABI changes

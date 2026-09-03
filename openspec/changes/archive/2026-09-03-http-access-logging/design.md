## Context
No logging middleware exists. The server uses slog for application logging but has no HTTP access logs.

## Goals
1. Log every meaningful HTTP request with structured fields
2. Filter noise (health checks, static assets)
3. Optional request ID for tracing

## Decisions

### D1: Chi middleware
Implement as a chi middleware function. Wraps `next.ServeHTTP` with a response writer capture (status + size), measures latency, and logs after the handler returns.

### D2: slog integration
Use the existing `slog.Default()` logger. INFO for 2xx/3xx, WARN for 4xx, ERROR for 5xx. Fields: method, path, status, size_bytes, latency_ms, request_id.

### D3: Noise filter via path prefix/suffix
Skip logging for paths matching `/health`, `/ready`, and file extensions `.js`, `.css`, `.webp`, `.br`, `.png`, `.jpg`, `.svg`, `.woff2`. Configurable via a skip function.

### D4: Request ID
Use `crypto/rand` for 8-char hex ID. Middleware sets `X-Request-Id` response header. Preserves client-provided ID.

## Risks
- High-traffic logging may impact performance — mitigated by noise filtering and slog's async-friendly design
- Request ID adds 8 bytes per response header — negligible
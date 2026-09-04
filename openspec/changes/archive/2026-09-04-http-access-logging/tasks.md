## Tasks

### 1. Logging middleware
- [x] 1.1 Create `internal/httpserver/middleware.go` with `loggingMiddleware` function
- [x] 1.2 Capture response status and size via wrapper ResponseWriter
- [x] 1.3 Measure latency and log with slog (INFO/WARN/ERROR by status)
- [x] 1.4 Add request ID generation and `X-Request-Id` header

### 2. Noise filtering
- [x] 2.1 Add skip function for health checks and static asset paths
- [x] 2.2 Wire skip function into middleware

### 3. Wire into router
- [x] 3.1 Add middleware to chi router in `routes.go`
- [x] 3.2 Ensure middleware runs before route handlers

### 4. Verify
- [x] 4.1 `go build ./... && go vet ./...` passes
- [x] 4.2 `go test ./internal/httpserver/` passes
- [x] 4.3 Live smoke: make requests, verify structured log output
- [x] 4.4 Verify health check and static assets are not logged
## Tasks

### 1. Logging middleware
- [ ] 1.1 Create `internal/httpserver/middleware.go` with `loggingMiddleware` function
- [ ] 1.2 Capture response status and size via wrapper ResponseWriter
- [ ] 1.3 Measure latency and log with slog (INFO/WARN/ERROR by status)
- [ ] 1.4 Add request ID generation and `X-Request-Id` header

### 2. Noise filtering
- [ ] 2.1 Add skip function for health checks and static asset paths
- [ ] 2.2 Wire skip function into middleware

### 3. Wire into router
- [ ] 3.1 Add middleware to chi router in `routes.go`
- [ ] 3.2 Ensure middleware runs before route handlers

### 4. Verify
- [ ] 4.1 `go build ./... && go vet ./...` passes
- [ ] 4.2 `go test ./internal/httpserver/` passes
- [ ] 4.3 Live smoke: make requests, verify structured log output
- [ ] 4.4 Verify health check and static assets are not logged
## Tasks

### 1. Test infrastructure
- [ ] 1.1 Create `internal/httpserver/testutil_test.go` with mockBridge struct
- [ ] 1.2 Implement mock methods for all bridge service interface methods
- [ ] 1.3 Create `newTestServer` helper with chi router + all routes

### 2. API endpoint tests
- [ ] 2.1 Create `internal/httpserver/api_test.go`
- [ ] 2.2 Table-driven tests for all 11 API endpoints (success, auth, error cases)

### 3. View handler tests
- [ ] 3.1 Create `internal/httpserver/views_test.go`
- [ ] 3.2 Tests for viewSearch, viewMangaDetail, viewHistory, viewLibrary, viewSettings, viewPlugins

### 4. Action handler tests
- [ ] 4.1 Create `internal/httpserver/actions_test.go`
- [ ] 4.2 Tests for all POST action handlers (toggle, mark-read, reset, export, clear, sync, save)

### 5. Sandbox endpoint tests
- [ ] 5.1 Create `internal/httpserver/sandbox_test.go`
- [ ] 5.2 Tests for load, unload, reload, call, cdp-status, cdp-test, cdp-cookies

### 6. Verify
- [ ] 6.1 `go test ./internal/httpserver/` passes
- [ ] 6.2 Coverage report shows >60% httpserver package coverage

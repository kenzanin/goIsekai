## Context
httpserver package contains all user-facing HTTP logic but has 6.8% test coverage. The bridge service interface is the main dependency to mock.

## Goals
1. Test infrastructure with httptest + chi + mock bridge
2. Table-driven tests for all endpoint groups
3. Target >60% httpserver package coverage

## Decisions

### D1: Mock bridge service
Create a `mockBridge` struct implementing the bridge service interface. Each method has a configurable return value set per test. Methods: Search, GetMangaDetail, GetChapterList, GetPageList, ToggleLibrary, MarkRead, etc.

### D2: Test helper function
`newTestServer(t *testing.T, mock *mockBridge) *httptest.Server` creates a chi router, registers all routes, and returns the test server. Reused across all test files.

### D3: Table-driven pattern
Each test function uses `[]struct{ name, method, path, wantStatus, wantBody }` pattern. Iterates and makes HTTP requests against the test server.

### D4: Template rendering in tests
View tests verify HTTP status and presence of key HTML markers (e.g., `class="manga-card"`) rather than exact HTML matching. Fragile exact matching avoided.

### D5: Separate test files per handler group
- `api_test.go` — API endpoints
- `views_test.go` — view handlers
- `actions_test.go` — action handlers
- `sandbox_test.go` — sandbox endpoints
- `testutil_test.go` — shared mock + helper

## Risks
- Mock maintenance as bridge interface grows — mitigated by compile-time interface check
- Template rendering tests may be brittle — use marker-based assertions

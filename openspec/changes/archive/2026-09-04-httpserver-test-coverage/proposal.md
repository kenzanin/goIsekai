## Why

httpserver has 6.8% test coverage despite containing all user-facing logic — handlers, views, actions, API, sandbox, reader-data. Every bug so far (qrm alias trap, chapter order, status normalization) was caught by manual browser testing. Without automated tests, regressions ship silently.

## What Changes

- Add table-driven handler tests for all httpserver endpoints using `httptest.NewServer` + chi router
- Mock the bridge service interface to isolate handler logic from plugin/DB dependencies
- Cover: api.go (11 endpoints), views.go (3 view handlers), actions.go (8 POST handlers), reader.go (readerData assembly), sandbox.go (4 endpoints)
- Target: >60% httpserver package coverage

## Capabilities

### New Capabilities

- `httpserver-test`: Automated test coverage for all HTTP handlers, views, actions, and API endpoints

### Modified Capabilities

(none)

## Impact

- `internal/httpserver/*_test.go` — new test files for each handler group
- No production code changes — tests only
- CI confidence for future httpserver changes

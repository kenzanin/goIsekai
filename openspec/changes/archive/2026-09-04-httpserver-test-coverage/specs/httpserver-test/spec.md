## Purpose

Add automated test coverage for all httpserver handlers, views, actions, and API endpoints to prevent regressions and catch bugs before manual testing.

## ADDED Requirements

### Requirement: Handler test infrastructure
A test helper SHALL provide `httptest.NewServer` with a chi router wired to mock bridge service implementations. The mock SHALL implement the bridge service interface used by all handlers.

#### Scenario: Test server setup
- **WHEN** a test function creates the test server
- **THEN** it returns an `httptest.Server` with all routes registered and a mock bridge

### Requirement: API endpoint tests
All 11 API endpoints (library, search, manga detail, history, toggle, read, progress, image, health, reader-data, plugins) SHALL have table-driven tests covering success, error, and edge cases.

#### Scenario: API library endpoint
- **WHEN** GET /api/library is called with valid API key
- **THEN** it returns 200 with JSON library data

#### Scenario: API library unauthorized
- **WHEN** GET /api/library is called without API key (when key is set)
- **THEN** it returns 401

### Requirement: View handler tests
View handlers (viewSearch, viewMangaDetail, viewHistory, viewLibrary, viewSettings, viewPlugins) SHALL have tests verifying correct template rendering and data assembly.

#### Scenario: Search view renders results
- **WHEN** GET /search?q=naruto is called
- **THEN** the response contains rendered HTML with search results

### Requirement: Action handler tests
All POST action handlers (toggle-library, mark-read, mark-read-range, reset-progress, export-cbz, clear-cache, sync, save-settings) SHALL have tests verifying correct behavior and redirect responses.

#### Scenario: Toggle library adds manga
- **WHEN** POST /actions/toggle-library with manga ID
- **THEN** the manga is added to library and response is 303 redirect

### Requirement: Sandbox endpoint tests
Sandbox endpoints (load, unload, reload, call, cdp-status, cdp-test, cdp-cookies) SHALL have tests with mock plugin manager.

#### Scenario: Sandbox load plugin
- **WHEN** POST /sandbox/load with plugin path
- **THEN** the plugin is loaded and response contains plugin ID

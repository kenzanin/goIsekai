# HTTP Server Specification

## Purpose

Provides a Chi HTTP server that serves Jet-rendered HTML fragments driven by HTMX, replacing the Wails desktop window with a browser-based UI.

## Requirements

### Requirement: HTTP server listens on configurable host and port
The system SHALL start a Chi HTTP server on the host and port specified by the `--host`/`--port` CLI flags or the `host`/`port` keys in `goisekai.ini` (default: `127.0.0.1:8080`). CLI flags override config values.

#### Scenario: Default host and port
- **WHEN** no `--host`/`--port` flags and no `host`/`port` config keys are provided
- **THEN** the server listens on `127.0.0.1:8080`

#### Scenario: Custom host and port via CLI flags
- **WHEN** the user runs `./goisekai --host 0.0.0.0 --port 9090`
- **THEN** the server listens on `0.0.0.0:9090`

#### Scenario: Custom host and port via config
- **WHEN** `goisekai.ini` contains `host = 0.0.0.0` and `port = 3000` under `[app]`
- **THEN** the server listens on `0.0.0.0:3000`

#### Scenario: CLI flags override config
- **WHEN** `goisekai.ini` contains `host = 0.0.0.0` `port = 3000` and the user runs `./goisekai --host 127.0.0.1 --port 9090`
- **THEN** the server listens on `127.0.0.1:9090`

### Requirement: Static file serving
The system SHALL serve static assets (CSS, JS, images, fonts) from `cmd/goisekai/frontend/` embedded via `go:embed` at `/static/`. The system SHALL serve the main layout template at `/` which includes Alpine.js and Tailwind CSS.

#### Scenario: Root path serves main layout
- **WHEN** a browser navigates to `http://localhost:8080/`
- **THEN** the server renders the main layout Lua template with navigation and default view (library)

#### Scenario: Static asset serving
- **WHEN** a browser requests `http://localhost:8080/static/lib/tailwind.css`
- **THEN** the server responds with the CSS file contents and `Content-Type: text/css`

### Requirement: HTMX/SPA HTML fragment endpoints
The system SHALL serve each view as a full HTML page or as a partial `<main>` content extract (determined by an `X-Partial` header). Each endpoint SHALL execute a Lua template and return HTML. The system SHALL support the following views: library, search, detail (manga chapters), reader, plugins, settings, logs.

#### Scenario: Full page render
- **WHEN** a browser navigates to `GET /view/library` without partial headers
- **THEN** the server renders the complete page (layout + view) using Lua templates

#### Scenario: Partial content extract for SPA navigation
- **WHEN** a client sends `GET /view/library` with `X-Partial: true` header
- **THEN** the server renders only `<main>` content (no layout wrapper) for client-side DOM morph

#### Scenario: Search view
- **WHEN** a client requests `GET /view/search?q=one+piece`
- **THEN** the server renders search results via Lua template

#### Scenario: Detail view
- **WHEN** a client requests `GET /view/manga/mangadex/{mangaID}`
- **THEN** the server renders manga details and chapter list via Lua template

### Requirement: HTMX form/action endpoints
The system SHALL handle form submissions and actions via POST endpoints. Each endpoint SHALL perform the action and respond with JSON (status + optional data) instead of HTML fragments. The client-side Alpine.js components SHALL handle response display via toast notifications.

#### Scenario: Install plugin
- **WHEN** a client sends `POST /action/install-plugin` with plugin file data
- **THEN** the server installs the plugin and returns `{"status": "ok"}` JSON

#### Scenario: Toggle plugin
- **WHEN** a client sends `POST /action/toggle-plugin/{pluginID}`
- **THEN** the server toggles the plugin state and returns `{"status": "ok", "active": true/false}` JSON

#### Scenario: Toggle library item
- **WHEN** a client sends `POST /action/toggle-library/{pluginID}/{mangaID}`
- **THEN** the server toggles the library item and returns `{"status": "ok", "in_library": true/false}` JSON

#### Scenario: Sync library
- **WHEN** a client sends `POST /action/sync`
- **THEN** the server syncs all library items and returns `{"status": "ok"}` JSON

### Requirement: Binary image endpoint
The system SHALL provide a dedicated endpoint `GET /image` that returns image bytes as a binary response. The system SHALL accept query parameters `pluginID`, `url`, `mangaID`, `chapterID` and request headers for proxy forwarding.

#### Scenario: Successful image fetch
- **WHEN** the browser requests `GET /image?pluginID=mangadex&url=https://...`
- **THEN** the server responds with `200 OK` and the raw image bytes with correct Content-Type

#### Scenario: Image from disk cache
- **WHEN** the requested image exists in the disk cache
- **THEN** the response serves the cached bytes without making a network request

### Requirement: WebSocket log streaming
The system SHALL provide a WebSocket endpoint at `GET /api/logs/ws` that streams log entries in real-time. Each message SHALL be a JSON object with `level`, `message`, and `time` fields. The system SHALL also provide `GET /api/logs` (HTTP GET) to retrieve the current log buffer as a JSON array.

#### Scenario: WebSocket connection
- **WHEN** the frontend opens a WebSocket connection to `ws://localhost:8080/api/logs/ws`
- **THEN** the server streams new log entries as they are generated

#### Scenario: HTTP log retrieval
- **WHEN** the frontend sends `GET /api/logs`
- **THEN** the server responds with a JSON array of buffered log entries

### Requirement: Auto-open browser
The system SHALL provide a `--open` CLI flag that automatically opens the default browser to `http://localhost:{port}` on startup.

#### Scenario: Auto-open enabled
- **WHEN** the user runs `./goisekai --open`
- **THEN** the system starts the server and opens `http://localhost:8080` in the default browser

#### Scenario: Auto-open with custom port
- **WHEN** the user runs `./goisekai --open --port 9090`
- **THEN** the system opens `http://localhost:9090` in the default browser

### Requirement: Optional API-key middleware on /api routes
When an API key is configured (`-apiKey` flag or `api_key` config key), the server SHALL mount an authentication middleware on `/api/*` (excluding `GET /image`) that rejects requests lacking a matching `X-API-Key` header with `401` and the JSON error envelope. When no key is configured, the middleware SHALL pass all requests through unchanged.

#### Scenario: Configured key rejects bad header
- **WHEN** an API key is configured and a request hits `/api/library` without `X-API-Key`
- **THEN** the server responds `401` with `{"error": "unauthorized"}` before any handler logic runs

#### Scenario: Unconfigured key passes through
- **WHEN** no API key is configured
- **THEN** requests to `/api/*` reach their handlers regardless of headers

### Requirement: JSON and view handlers share one service layer
Each `/api/*` handler SHALL obtain its data by calling the same bridge service methods as its corresponding view handler. Introducing a JSON endpoint SHALL NOT duplicate domain logic or query code.

#### Scenario: Data parity between view and API
- **WHEN** the library state changes and a client compares `GET /view/library` (parsed HTML) with `GET /api/library`
- **THEN** both reflect the same underlying library entries from the same service call



## MODIFIED Requirements

### Requirement: Static file serving
The system SHALL serve static assets (CSS, JS, images, fonts) from `cmd/goisekai/frontend/` embedded via `go:embed` at `/static/`. The system SHALL serve the main layout template at `/` which includes Alpine.js and Tailwind CSS.

#### Scenario: Root path serves main layout
- **WHEN** a browser navigates to `http://localhost:8080/`
- **THEN** the server renders the main layout Lua template with navigation and default view (library)

#### Scenario: Static asset serving
- **WHEN** a browser requests `http://localhost:8080/static/lib/tailwind.css`
- **THEN** the server responds with the CSS file contents and `Content-Type: text/css`

### Requirement: HTMX HTML fragment endpoints
The system SHALL serve each view as a full HTML page or as a partial `<main>` content extract (determined by an `X-Partial` header). Each endpoint SHALL execute a Lua template and return HTML. The system SHALL support the following views: library, search, detail (manga chapters), reader, plugins, settings, logs.

#### Scenario: Full page render
- **WHEN** a browser navigates to `GET /view/library` without partial headers
- **THEN** the server renders the complete page (layout + view) using Lua templates

#### Scenario: Partial content extract for SPA navigation
- **WHEN** a client sends `GET /view/library` with `X-Partial: true` header
- **THEN** the server renders only the `<main>` content (no layout wrapper) for client-side DOM morph

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

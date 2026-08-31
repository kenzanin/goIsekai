# Bridge Specification

## Purpose

Defines the bridge service's HTMX integration contract — how the frontend communicates with the Go backend via HTML fragment endpoints rendered by Jet templates.

## Requirements

### Requirement: Bridge service method exposure via HTMX
The bridge service methods SHALL be exposed via Chi HTTP routes that render Jet templates. Each view endpoint SHALL return an HTML fragment (for HTMX `hx-get`) or a full page (for direct navigation). The system SHALL use Jet template inheritance (extends/block/yield) for consistent layout.

#### Scenario: View endpoint returns HTML fragment
- **WHEN** HTMX sends `GET /view/library` with `HX-Request: true` header
- **THEN** the bridge renders the library Jet template with manga data and returns an HTML fragment

#### Scenario: View endpoint returns full page
- **WHEN** a browser navigates to `GET /view/library` without HTMX headers
- **THEN** the bridge renders the full page layout (extends base template) with the library view

#### Scenario: Action endpoint returns updated fragment
- **WHEN** HTMX sends `POST /action/toggle-plugin/{pluginID}`
- **THEN** the bridge toggles the plugin state and returns the updated plugin card HTML fragment

### Requirement: Image data transfer
The `GetImage` method SHALL return image bytes as a binary HTTP response. The endpoint SHALL be `GET /image` with query parameters for `pluginID`, `url`, `mangaID`, `chapterID`.

#### Scenario: Successful image transfer
- **WHEN** the browser requests `GET /image?pluginID=mangadex&url=https://...`
- **THEN** the response contains raw image bytes with the correct `Content-Type` header

#### Scenario: Image from disk cache
- **WHEN** the requested image exists in the disk cache
- **THEN** the response serves the cached bytes without making a network request

### Requirement: Log streaming via WebSocket
The bridge SHALL stream log entries to connected WebSocket clients at `GET /api/logs/ws`. Each message SHALL be a JSON object with `level`, `message`, and `time` fields. The bridge SHALL also provide `GET /api/logs` to retrieve the current log buffer.

#### Scenario: Live log streaming
- **WHEN** a WebSocket client is connected and a new log entry is generated
- **THEN** the entry is pushed to all connected clients as a JSON message

#### Scenario: Log buffer retrieval
- **WHEN** the frontend sends `GET /api/logs`
- **THEN** the response contains the current log buffer as a JSON array

### Requirement: Reader view with canvas rendering
The reader view SHALL use a Jet template that includes a canvas element and vanilla JavaScript for canvas rendering (zoom/pan/drag). The reader SHALL NOT use HTMX for canvas interactions — these remain client-side JavaScript. HTMX SHALL be used only for chapter navigation (next/prev chapter triggers a new view load).

#### Scenario: Reader loads chapter
- **WHEN** HTMX sends `GET /view/read/{pluginID}/{mangaID}/{chapterID}`
- **THEN** the server renders the reader Jet template with page list data and canvas JavaScript

#### Scenario: Chapter navigation via HTMX
- **WHEN** the user clicks "Next Chapter" in the reader
- **THEN** HTMX sends `GET /view/read/{pluginID}/{mangaID}/{nextChapterID}` and swaps the reader content

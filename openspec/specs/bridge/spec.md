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

### Requirement: Image priority parameter

The image endpoint (`GET /image`) SHALL accept an optional `prio` query
parameter with value `high` or `low`. The resolved priority SHALL be
passed through to the image fetch pipeline. Absent or unrecognized
values SHALL resolve to `low`.

#### Scenario: Reader marks the on-screen page high priority

- **WHEN** the reader requests the page currently displayed with
  `prio=high`
- **THEN** the fetch pipeline treats that request as high priority

#### Scenario: Reader marks read-ahead pages low priority

- **WHEN** the reader requests read-ahead or neighbor-chapter pages with
  `prio=low` (or without the parameter)
- **THEN** the fetch pipeline treats those requests as low priority

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

### Requirement: Manga detail page renders from persisted data when the plugin is unreachable

When the live plugin fetch for a manga fails with an error other than an
anti-bot challenge, the detail page SHALL retry with the manga's persisted copy
(`CachedMangaAndChapters` plus the enrichment-derived fields the live path
fills: stored author, override genres, cover dimensions, alt titles, alt
summaries, categories, related entries). If the persisted copy exists, the page
SHALL render normally with a visible cached-data notice. If neither the plugin
nor a persisted copy exists, the server SHALL return the existing error
response.

#### Scenario: Detail page opens while offline

- **WHEN** the plugin for the requested manga cannot be reached
- **AND** the manga has a persisted copy in the database
- **THEN** the detail page renders with the persisted title, cover, description,
  status, author, genres, enrichment rows, and chapter list
- **AND** the page shows a cached-data notice

#### Scenario: Detail page for an unknown manga while offline

- **WHEN** the plugin cannot be reached
- **AND** the manga has no persisted copy
- **THEN** the server responds with the existing error page

#### Scenario: Online loads show no notice

- **WHEN** the detail page loads via a successful plugin fetch
- **THEN** no cached-data notice is rendered

### Requirement: Plugin response caching

The plugin manager SHALL be wired to the application database at startup, and
plugin responses for `GetMangaDetail` and `GetChapterList` SHALL be stored in
`plugin_cache` and served from it while unexpired. `GetMangaDetail` responses
SHALL use the `cache_ttl_hours` value (default 24); `GetChapterList` responses
SHALL use the `chapter_cache_ttl_hours` value (default 168).

#### Scenario: Second detail page load serves from cache

- **WHEN** a manga's detail page is loaded twice within the TTL
- **THEN** the second load does not invoke the plugin for `GetMangaDetail`
- **AND** the response equals the first load's data

#### Scenario: Chapter list survives past 24 hours

- **WHEN** a chapter list was cached 25 hours ago
- **AND** `chapter_cache_ttl_hours` is 168
- **THEN** the next chapter list request is served from `plugin_cache`

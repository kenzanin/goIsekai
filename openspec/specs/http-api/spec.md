# HTTP API Specification

## Purpose

Exposes goIsekai's domain data as a machine-facing JSON API under `/api/` so external clients (custom UIs, scripts, wrappers) can consume the same bridge service layer as the server-rendered UI, with consistent error shapes and an optional API-key gate for non-loopback exposure.

## Requirements

### Requirement: JSON API mirrors view endpoints
The system SHALL expose JSON endpoints under `/api/` that return the same data the corresponding view endpoints render, sourced from the same bridge service calls. The initial inventory SHALL include: library list, search, manga detail, chapter list, history, reader-data (existing), toggle-library, mark-chapter-read, and set-progress.

#### Scenario: Library as JSON
- **WHEN** a client sends `GET /api/library`
- **THEN** the response is `200` with a JSON array of library entries containing title, plugin ID, source manga ID, cover URL, progress stats, and the `new_since` badge state

#### Scenario: Search as JSON
- **WHEN** a client sends `GET /api/search?pluginID=mangzio&q=isekai&page=1`
- **THEN** the response is `200` with a JSON object containing the results for that page and a `has_next` boolean computed the same way as the view pagination

#### Scenario: Manga detail as JSON
- **WHEN** a client sends `GET /api/manga/{pluginID}/{mangaID}`
- **THEN** the response is `200` with JSON containing title, status, description, cover URL, chapter list (newest-first), per-chapter progress, and the continue point

#### Scenario: View routes unchanged
- **WHEN** any `/view/*` or `/action/*` route is requested after this change
- **THEN** it behaves exactly as before (HTML rendering, 303 redirects for actions) with no behavioral change

### Requirement: Consistent JSON error envelope
Every `/api/*` error response SHALL be a JSON object of the form `{"error": "<message>"}` with an appropriate HTTP status: `400` for missing/invalid parameters, `404` for an unknown resource, and `502` when an upstream plugin call fails.

#### Scenario: Missing parameter
- **WHEN** a client calls `GET /api/search` without `q`
- **THEN** the response is `400` with body `{"error": "..."}` describing the missing parameter

#### Scenario: Unknown manga
- **WHEN** a client calls `GET /api/manga/{pluginID}/{mangaID}` for a manga that is not in the library and cannot be fetched
- **THEN** the response is `404` with a JSON error body, not an HTML error page

### Requirement: Binary image payload endpoint
The system SHALL expose `GET /api/image/{pluginID}/{mangaID}/{chapterID}?url=<encoded>` returning the raw image bytes (the same bytes the `/image` view endpoint serves, including WebP conversion and two-level validation with refetch semantics). When validation of cached or fetched bytes fails, the handler SHALL refetch from the original source once and serve the revalidated bytes; on final failure it SHALL respond `502` with the JSON error envelope.

#### Scenario: Payload fetch succeeds
- **WHEN** a client requests a valid image through the API image endpoint
- **THEN** the response is `200` with `Content-Type` set to the image type and the raw bytes in the body — no JSON wrapper, no link references

#### Scenario: Source failure after refetch
- **WHEN** both fetch attempts fail image validation
- **THEN** the response is `502` with body `{"error": "..."}` and nothing is written to cache

### Requirement: Optional API key for /api routes
The system SHALL accept an API key from the `-apiKey` CLI flag or `api_key` config key. When set, every `/api/*` request (except `GET /image`) SHALL require header `X-API-Key` matching the configured value and SHALL reject others with `401` and the JSON error envelope. When unset, `/api/*` SHALL be served without an API-key check.

#### Scenario: Key required and valid
- **WHEN** an API key is configured and a client sends `GET /api/library` with header `X-API-Key` matching it
- **THEN** the response is `200` with the normal JSON body

#### Scenario: Key missing or wrong
- **WHEN** an API key is configured and a client calls any `/api/*` route without the header or with a wrong value
- **THEN** the response is `401` with body `{"error": "unauthorized"}`

#### Scenario: No key configured
- **WHEN** no API key is configured
- **THEN** `/api/*` routes respond without any key check (loopback-trust behavior unchanged)

#### Scenario: Image endpoint exempt
- **WHEN** an API key is configured and a browser requests `GET /image?...` without the `X-API-Key` header

### Requirement: Alt-title server discovery endpoint
The system SHALL expose `GET /api/alt-title-servers` returning a JSON array of `{provider_plugin, server_id, name}` entries aggregated from all installed plugins declaring the alt-title enricher capability. It SHALL return `200` with an empty array when no provider exists.

#### Scenario: List servers
- **WHEN** a client sends `GET /api/alt-title-servers` while the MangaDex plugin is installed and exposes one server
- **THEN** the response is `200` with one entry `{provider_plugin: "mangadex", server_id: "mangadex", name: "MangaDex"}`

### Requirement: Alt-title management endpoints
The system SHALL expose alt-title management under `/api/manga/{pluginID}/{mangaID}/alt-titles`: `POST` with `{"server"}` fetches from the provider plugin and merges results (skipping duplicates), and `DELETE` with `{"title"}` removes one stored row. Removal of the current main title SHALL be rejected with a validation error.

#### Scenario: Fetch merges without duplicates
- **WHEN** `POST /api/manga/p/m/alt-titles` is called twice with the same server and identical results
- **THEN** the second call stores no duplicate rows and responds with the merged list

#### Scenario: Delete a stored title
- **WHEN** `DELETE /api/manga/p/m/alt-titles` is called with a stored title
- **THEN** the response is `200` and the title no longer appears in subsequent listings

### Requirement: Main title update endpoint
The system SHALL expose `PUT /api/manga/{pluginID}/{mangaID}/title` with `{"title"}`. The title MUST exist in the manga's stored alternative titles; on success the main title is swapped (old main title inserted into the alternative list). Invalid titles SHALL return a validation error with the main title unchanged.

#### Scenario: Swap main title
- **WHEN** `PUT /api/manga/p/m/title` is called with a stored alternative title
- **THEN** the response is `200` and the manga's main title equals the requested value while the old main title appears among the alternatives

### Requirement: Library search endpoint
The system SHALL expose `GET /api/library/search?q=<query>` returning a JSON array of `{plugin_id, manga_id, title, score}` ranked by typo-tolerant relevance over main titles and stored alternative titles, using the full-text index for candidate retrieval.

#### Scenario: Search by alternative title
- **WHEN** `GET /api/library/search?q=` is called with a string equal to a stored alternative title
- **THEN** the response is `200` and contains that manga, ranked at or above substring matches

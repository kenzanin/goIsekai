# HTTP API Specification (delta)

## ADDED Requirements

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

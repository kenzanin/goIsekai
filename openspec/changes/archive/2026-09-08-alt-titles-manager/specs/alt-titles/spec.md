# alt-titles

## Purpose

Resolve, store, and curate alternative titles for library manga, sourced from plugin-declared lookup servers, enabling reliable duplicate detection and library search at scale.

## ADDED Requirements

### Requirement: Provider discovery
The system shall expose a JSON list of all available alt-title lookup servers, aggregated from plugins that declare the alt-title enricher capability. The list shall contain, per entry, the provider plugin identifier, the server identifier, and a display name. The host shall not hardcode any provider or server.

#### Scenario: Plugin declares multiple servers
- **WHEN** a plugin declares two lookup servers in its metadata
- **THEN** `GET /api/alt-title-servers` returns both entries tagged with that plugin's id

#### Scenario: No provider installed
- **WHEN** no active plugin exposes the alt-title capability
- **THEN** `GET /api/alt-title-servers` returns an empty list and no error

### Requirement: Fetch alternative titles
The system shall fetch alternative titles for a library manga by delegating to the provider plugin's lookup function with the user-selected server identifier and the manga's current title, and shall merge the returned titles into persistent storage tagged with the provider-reported source label. Titles already stored for that manga shall be skipped (no duplicates). An empty result shall not delete existing rows.

#### Scenario: Fetch with a chosen server
- **WHEN** `POST /api/manga/{pluginID}/{mangaID}/alt-titles` is called with a server id offered by an installed provider
- **THEN** new titles returned by the provider are stored with their source label and pre-existing titles are unchanged

#### Scenario: Unknown server
- **WHEN** the request names a server id not present in the aggregated server list
- **THEN** the API responds with an error indicating the server is unavailable

### Requirement: List alternative titles
The system shall return the stored alternative titles for a manga, each with its source label, alongside the manga's main title.

#### Scenario: Detail data includes alt titles
- **WHEN** the manga detail view or its API is requested for a manga with stored alt titles
- **THEN** the alternative titles and their source labels are included in the response

### Requirement: Set main title from alternative list
The system shall allow setting a manga's main library title to exactly one of its stored alternative titles. On success the chosen title becomes the main title, and the previous main title is inserted into the alternative list. Titles not present in the alternative list shall be rejected.

#### Scenario: Promote an alternative title
- **WHEN** `PUT /api/manga/{pluginID}/{mangaID}/title` is called with a title present in the manga's stored alternatives
- **THEN** the manga's main title becomes the requested value and the previous main title appears in the alternative list

#### Scenario: Reject unknown title
- **WHEN** the requested title is not in the stored alternative list
- **THEN** the API responds with a validation error and the main title is unchanged

### Requirement: Remove alternative title
The system shall allow removing a single stored alternative title for a manga.

#### Scenario: Remove one title
- **WHEN** `DELETE /api/manga/{pluginID}/{mangaID}/alt-titles` is called with a stored title
- **THEN** that row is deleted and other alternative titles remain

#### Scenario: Main title protected
- **WHEN** removal is attempted for a title equal to the current main title
- **THEN** the request is rejected with a validation error

### Requirement: Library fuzzy search
The system shall provide library search over main titles and stored alternative titles, using a SQLite full-text index for candidate retrieval and typo-tolerant ranking (exact match ranked above prefix, prefix above substring, substring above subsequence). Results shall include plugin id, manga id, title, and relevance score, and shall function performantly with 2000+ titles.

#### Scenario: Search hits alt title
- **WHEN** the user searches a string matching a stored alternative title but not the main title
- **THEN** the manga is returned in results

#### Scenario: Typo tolerance
- **WHEN** the query contains a minor typo that is a subsequence of a title (e.g. "iseki" for "isekai")
- **THEN** the matching manga is still returned, ranked below exact/prefix matches

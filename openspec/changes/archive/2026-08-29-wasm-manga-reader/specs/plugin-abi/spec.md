## Purpose

Defines the versioned data contract and the host-function interface shared between the host application and every manga source plugin, so host and plugins can evolve independently against a stable ABI.

## ADDED Requirements

### Requirement: Manga data contract
The system SHALL represent a manga using a JSON-serializable DTO containing `id`, `title`, `cover_url`, and optional `author`, `description`, `status`, and `genres`.

#### Scenario: Serialize manga with full metadata
- **WHEN** a plugin returns a manga with title, cover URL, author, description, status, and genres
- **THEN** the host parses it into a `Manga` value with all fields preserved
- **AND** omitted optional fields remain empty rather than erroring

#### Scenario: Serialize minimal manga
- **WHEN** a plugin returns a manga with only `id`, `title`, and `cover_url`
- **THEN** the host accepts it without requiring optional fields

### Requirement: Chapter data contract
The system SHALL represent a chapter using a DTO containing `id`, `manga_id`, `title`, `chapter_num`, `released_at`, `url`, and optional `volume_num`.

#### Scenario: Serialize chapter
- **WHEN** a plugin returns a chapter with a numeric chapter number and release timestamp
- **THEN** the host parses `chapter_num` as a number and `released_at` as a timestamp

### Requirement: Page data contract
The system SHALL represent a page using a DTO containing an ordered `index`, a `url`, and optional custom `headers` (e.g. per-page Referer/User-Agent overrides).

#### Scenario: Serialize page with custom headers
- **WHEN** a plugin returns a page whose image requires a custom Referer
- **THEN** the host preserves the page's `headers` map for use when fetching the image

### Requirement: Search filter contract
The system SHALL represent a search request using a DTO containing `query`, `page`, and optional `genres` and `sort_by`.

#### Scenario: Serialize search filter
- **WHEN** the frontend searches with a query, page number, genre list, and sort preference
- **THEN** the host serializes all fields into the JSON passed to the plugin

### Requirement: Plugin search host function
Every plugin SHALL expose a `Search` host function that accepts `filter_json` and returns `result_json`.

#### Scenario: Search returns results
- **WHEN** the host calls `Search` with a valid filter JSON
- **THEN** the plugin returns a JSON array of manga results

### Requirement: Plugin manga-detail host function
Every plugin SHALL expose a `GetMangaDetail` host function that accepts `manga_id` and returns `manga_json`.

#### Scenario: Fetch manga detail
- **WHEN** the host calls `GetMangaDetail` with a manga id
- **THEN** the plugin returns the manga's JSON representation

### Requirement: Plugin chapter-list host function
Every plugin SHALL expose a `GetChapterList` host function that accepts `manga_id` and returns `chapters_json`.

#### Scenario: Fetch chapter list
- **WHEN** the host calls `GetChapterList` with a manga id
- **THEN** the plugin returns a JSON array of chapters

### Requirement: Plugin page-list host function
Every plugin SHALL expose a `GetPageList` host function that accepts `chapter_id` and returns `pages_json`.

#### Scenario: Fetch page list
- **WHEN** the host calls `GetPageList` with a chapter id
- **THEN** the plugin returns a JSON array of pages in reading order

### Requirement: JSON-over-memory invocation
The host SHALL communicate with plugins via JSON strings passed over memory (not shared typed objects), and SHALL version the contract so mismatched host/plugin versions are detectable.

#### Scenario: Detect contract mismatch
- **WHEN** the host and a plugin report incompatible contract versions
- **THEN** the host rejects the plugin with an explicit error rather than mis-parsing its output

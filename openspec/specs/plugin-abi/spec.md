# Plugin ABI Specification

## Purpose

Defines the versioned data contract and the host-function interface shared between the host application and every manga source plugin, so host and plugins can evolve independently against a stable ABI.

## Requirements

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


### Requirement: Scriggo runtime kind

The plugin manager SHALL accept `"scriggo"` as a valid plugin runtime kind alongside `"wasm"`, `"lua"`, `"js"`, and `"go"`. Scriggo plugins SHALL implement the same ABI function contract (same function names, same JSON argument/return shapes) as all other runtime kinds.

#### Scenario: Scriggo plugin implements full ABI

- **WHEN** a Scriggo plugin exports `Init`, `SearchManga`, `GetMangaDetails`, `GetChapterList`, and `GetPageList`
- **THEN** the host accepts it as a valid plugin and dispatches ABI calls identically to other runtime kinds

#### Scenario: Scriggo plugin with alt-titles capability

- **WHEN** a Scriggo plugin also exports `GetAltTitles`
- **THEN** the host exposes its alt-title servers through the same capability-discovery mechanism as other runtimes

### Requirement: Alt-titles enricher contract
A plugin SHALL optionally declare an alt-title enricher capability by (a) listing its lookup servers as `alt_title_servers` (array of `{id, name}`) in its plugin metadata, and (b) exporting a `GetAltTitles` function accepting a JSON object `{"title": string, "server": string}` and returning `{"source": string, "titles": []string}`. The `source` value is the provider-defined badge label displayed to users. The host SHALL treat this function as optional: a plugin without it remains fully functional as a source plugin.

#### Scenario: Plugin with enricher capability

- **WHEN** a plugin's metadata contains a non-empty `alt_title_servers` array and it exports `GetAltTitles`
- **THEN** the host recognizes it as an alt-title provider and its servers appear in the aggregated server list

#### Scenario: Plugin without enricher capability

- **WHEN** a plugin omits `alt_title_servers` and does not export `GetAltTitles`
- **THEN** loading the plugin succeeds and it is excluded from the provider list

#### Scenario: GetAltTitles call shape

- **WHEN** the host invokes `GetAltTitles` with `{"title": "Solo Leveling", "server": "mangadex"}`
- **THEN** the plugin returns `{"source": "MangaDex", "titles": ["...", ...]}` with at minimum `source` and a possibly-empty `titles` array
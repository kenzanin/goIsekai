## Purpose

Provide host-managed enrichment of manga metadata from external providers — built-in MangaDex and MangaUpdates plus plugin-declared custom sources — across alternative titles, alternative summaries, categories, and related manga, fetched on demand and persisted for later display.

## ADDED Requirements

### Requirement: Built-in enrichment providers

The host SHALL provide built-in enrichment providers for MangaDex and MangaUpdates that function without any plugin installed. A built-in provider SHALL declare which enrichment kinds it supports:

- MangaDex: `titles`, `categories`, `related`
- MangaUpdates: `titles`, `summaries`, `categories`, `related`

#### Scenario: Built-in provider available with no plugins installed
- **WHEN** no plugin is active and the enrichment source catalog is requested
- **THEN** the catalog lists the MangaDex and MangaUpdates sources with their supported kinds

#### Scenario: Built-in provider resolves by title
- **WHEN** an enrichment fetch is requested for a source and kind the built-in provider supports, supplying a search title
- **THEN** the provider queries the external API using that title and returns items tagged with the provider's source label

### Requirement: Enrichment source catalog

The system SHALL expose a catalog of available enrichment sources aggregated from built-in providers and from active plugins that declare enrichment providers. Each catalog entry SHALL contain the source identifier, a display name, and the set of enrichment kinds the source supports. The catalog SHALL be filterable by kind.

#### Scenario: Catalog includes built-in and plugin sources
- **WHEN** the catalog is requested and one plugin declares a custom source for `titles`
- **THEN** the catalog contains the built-in MangaDex/MangaUpdates entries plus the plugin source entry with its supported kinds

#### Scenario: Catalog filtered by kind
- **WHEN** the catalog is requested for a specific kind (e.g. `categories`)
- **THEN** only entries whose supported kinds include that kind are returned

#### Scenario: No sources for a kind
- **WHEN** no provider supports the requested kind
- **THEN** the catalog returns an empty list without error

### Requirement: On-demand enrichment fetch

The system SHALL fetch enrichment items for a library manga by kind and source, resolve the manga's current title, dispatch to the resolved provider, and merge the returned items into persistent storage tagged with the provider-reported source label. Items already stored for that manga and kind SHALL be skipped (no duplicates). An empty provider result SHALL NOT delete existing rows.

#### Scenario: Fetch categories from a built-in source
- **WHEN** an enrichment fetch for kind `categories` and source `mangadex` is requested for a library manga
- **THEN** the provider's categories are persisted with the MangaDex source label and previously stored categories are unchanged

#### Scenario: Duplicate items are not stored twice
- **WHEN** a fetch returns an item already stored for that manga and kind
- **THEN** no duplicate row is created and the existing row is preserved

#### Scenario: Empty result preserves stored data
- **WHEN** a provider returns no items
- **THEN** the existing stored items for that manga and kind remain unchanged

#### Scenario: Unknown source or kind
- **WHEN** the requested source is not in the catalog or does not support the requested kind
- **THEN** the system responds with an error indicating the source/kind is unavailable and stores nothing

### Requirement: Category enrichment
The system SHALL store fetched categories (genres/tags) per manga with their source label, deduplicated per manga, and SHALL include the stored categories in the manga detail view data.

#### Scenario: Categories rendered on detail view
- **WHEN** the manga detail view is requested for a manga with stored categories
- **THEN** the categories and their source labels are included in the response

### Requirement: Related manga enrichment
The system SHALL store fetched related/recommended manga per manga with their source label, where each related item carries at least a title and either a link or a cover image reference, deduplicated per manga, and SHALL include the stored related items in the manga detail view data.

#### Scenario: Related items rendered on detail view
- **WHEN** the manga detail view is requested for a manga with stored related items
- **THEN** the related items and their source labels are included in the response

#### Scenario: Related items are deduplicated
- **WHEN** two providers return the same related title for a manga
- **THEN** only one related row is stored

### Requirement: Plugin-declared enrichment providers

The system SHALL allow a plugin to declare one or more enrichment providers in its metadata, each with a source identifier, a display name, and supported kinds, and SHALL invoke the plugin's enrichment export when a fetch targets a plugin-declared source. A plugin that declares no enrichment providers SHALL remain a fully functional source plugin. When a plugin-declared provider is unavailable or fails, the failure SHALL surface as an error for that fetch only and SHALL NOT prevent other sources from being used.

#### Scenario: Plugin-declared provider participates in catalog
- **WHEN** an active plugin declares an enrichment source supported for `titles`
- **THEN** the source appears in the catalog and is selectable for `titles` fetches

#### Scenario: Plugin without enrichment capability
- **WHEN** a plugin declares no enrichment providers
- **THEN** it loads and operates normally and contributes no catalog entries

#### Scenario: Plugin provider failure isolated
- **WHEN** a fetch against a plugin-declared source fails or times out
- **THEN** the fetch returns an error and built-in sources remain usable

### Requirement: Compact enrichment fetch panel

The manga detail view SHALL present a single enrichment control consisting of a kind selector, a source selector, and a submit action. The panel SHALL be collapsed by default and expandable by the user. The source selector SHALL offer only sources that support the currently selected kind.

#### Scenario: Panel collapsed by default
- **WHEN** the detail view first renders
- **THEN** the enrichment controls are collapsed and the detail content is shown without the fetch form expanded

#### Scenario: Source options follow selected kind
- **WHEN** the user selects a kind in the panel
- **THEN** the source selector lists only sources whose supported kinds include that kind

#### Scenario: Submit fetches and reflects result
- **WHEN** the user selects a kind and source and submits
- **THEN** the fetch runs for that combination and the resulting items appear in the corresponding detail section

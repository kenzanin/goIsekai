## ADDED Requirements

### Requirement: Enrichment script sources

The host SHALL treat each folder under the configured info directory (`info_dir`, default `<data_dir>/info`) that contains a `main.lua` as one enrichment source, using the folder name as the source identifier. A source SHALL declare its providers in `PLUGIN.enrichment_providers`, each with a source id, a display name, and the enrichment kinds it supports, and SHALL implement `getEnrichment` to serve a fetch. Adding, changing, or removing a source SHALL require no host rebuild: the info directory SHALL be rescanned at startup. An enrichment source SHALL NOT be usable as a manga source.

#### Scenario: Source discovered from a folder
- **WHEN** the host starts with `info_dir` containing a `mangadex/main.lua` that declares a provider for `titles`
- **THEN** `mangadex` appears in the enrichment catalog with the kinds it declared

#### Scenario: Source added without a rebuild
- **WHEN** a new folder containing a `main.lua` is added under `info_dir` and the host restarts
- **THEN** the new source is offered in the catalog with no host code change

#### Scenario: Enrichment source is not a manga source
- **WHEN** an enrichment source is discovered
- **THEN** it is absent from the plugin list, is not offered as a search target, and is not asked for chapter pages

#### Scenario: Missing info directory does not fail startup
- **WHEN** `info_dir` does not exist
- **THEN** the host starts normally with an empty enrichment catalog

### Requirement: Author enrichment

The system SHALL support `authors` as an enrichment kind, and SHALL store a fetched author on the manga rather than as a list of alternative rows. A fetched author SHALL replace the stored value, blank author names in a result SHALL be ignored, and the manga detail view SHALL use the stored author when the source plugin supplies none.

#### Scenario: Author stored on the manga
- **WHEN** an enrichment fetch for kind `authors` returns author names for a library manga
- **THEN** the manga's author is set to those names and a later fetch replaces the stored value

#### Scenario: Stored author fills a gap
- **WHEN** the detail view is requested for a manga whose source plugin reported no author and a stored author exists
- **THEN** the view shows the stored author

#### Scenario: Source plugin author wins
- **WHEN** the source plugin supplies an author while a stored author also exists
- **THEN** the view shows the plugin's author and the stored value is left untouched

## MODIFIED Requirements

### Requirement: Enrichment source catalog

The system SHALL expose a catalog of available enrichment sources aggregated from the enrichment sources discovered in the info directory and from active plugins that declare enrichment providers, including declarations from sources that have not yet been invoked. Each catalog entry SHALL contain the source identifier, a display name, and the set of enrichment kinds the source supports. The catalog SHALL be filterable by kind. Reading the catalog SHALL instantiate only the scripts needed to learn their declarations, leaving manga source plugins unloaded.

#### Scenario: Catalog includes built-in and plugin sources
- **WHEN** the catalog is requested and one info script and one plugin each declare a source for `titles`
- **THEN** the catalog contains both entries with their supported kinds

#### Scenario: Catalog filtered by kind
- **WHEN** the catalog is requested for a specific kind (e.g. `categories`)
- **THEN** only entries whose supported kinds include that kind are returned

#### Scenario: No sources for a kind
- **WHEN** no provider supports the requested kind
- **THEN** the catalog returns an empty list without error

#### Scenario: Catalog does not load source plugins
- **WHEN** the catalog is requested while a manga source plugin is registered but not yet invoked
- **THEN** no runtime is instantiated for it by reading the catalog alone

### Requirement: On-demand enrichment fetch

The system SHALL fetch enrichment items for a library manga by kind and source, resolve the manga's current title, dispatch to the resolved provider, and merge the returned items into persistent storage tagged with the provider-reported source label. Items already stored for that manga and kind SHALL be skipped (no duplicates). An empty provider result SHALL NOT delete existing rows.

#### Scenario: Fetch categories from a built-in source
- **WHEN** an enrichment fetch for kind `categories` and source `mangadex` is requested for a library manga and the `mangadex` source declares `categories`
- **THEN** the provider's categories are persisted with the `mangadex` source label and previously stored categories are unchanged

#### Scenario: Duplicate items are not stored twice
- **WHEN** a fetch returns an item already stored for that manga and kind
- **THEN** no duplicate row is created and the existing row is preserved

#### Scenario: Empty result preserves stored data
- **WHEN** a provider returns no items
- **THEN** the existing stored items for that manga and kind remain unchanged

#### Scenario: Unknown source or kind
- **WHEN** the requested source is not in the catalog or does not support the requested kind
- **THEN** the system responds with an error indicating the source/kind is unavailable and stores nothing

### Requirement: Plugin-declared enrichment providers

The system SHALL allow an enrichment script or a source plugin to declare one or more enrichment providers, each with a source identifier, a display name, and supported kinds, and SHALL invoke the declaring script's enrichment export when a fetch targets that source. A source plugin that declares no enrichment providers SHALL remain a fully functional manga source. When a declaring script is unavailable or fails, the failure SHALL surface as an error for that fetch only and SHALL NOT prevent other sources from being used.

#### Scenario: Plugin-declared provider participates in catalog
- **WHEN** an active script declares an enrichment source supported for `titles`
- **THEN** the source appears in the catalog and is selectable for `titles` fetches

#### Scenario: Plugin without enrichment capability
- **WHEN** a source plugin declares no enrichment providers
- **THEN** it loads and operates normally as a manga source and contributes no catalog entries

#### Scenario: Plugin provider failure isolated
- **WHEN** a fetch against a declared source fails or times out
- **THEN** the fetch returns an error and the remaining sources stay usable

## REMOVED Requirements

### Requirement: Built-in enrichment providers
**Reason**: Enrichment sources are no longer host code. The MangaDex and MangaUpdates provider implementations were deleted, so the host ships no provider that works without a script installed, and the per-provider kind list this requirement pinned is now declared by each script.
**Migration**: Install (or author) an enrichment script for each source that should remain available, under `info_dir` — one folder per source, entry `main.lua`, declaring `PLUGIN.enrichment_providers` and implementing `getEnrichment`. `examples/info/mangadex/main.lua` is a working reference. A source that is not replaced by a script stops appearing in the catalog, and fetches naming it return the unknown-source error.

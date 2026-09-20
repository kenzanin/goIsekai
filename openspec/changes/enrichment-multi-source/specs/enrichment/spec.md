## MODIFIED Requirements

### Requirement: Enrichment source catalog

The system SHALL expose a catalog of available enrichment sources aggregated from the enrichment sources discovered in the info directory and from active plugins that declare enrichment providers, including declarations from sources that have not yet been invoked. Each catalog entry SHALL contain the source identifier, a display name, the set of enrichment kinds the source supports, and the source's precedence. The catalog SHALL be filterable by kind. The catalog SHALL be ordered by precedence ascending, with the source identifier breaking ties, and that order SHALL be the order a fetch follows. Sources that are disabled SHALL NOT appear in the catalog, in a kind filter, or in a precedence order. Reading the catalog SHALL instantiate only the scripts needed to learn their declarations, leaving manga source plugins unloaded.

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

#### Scenario: Catalog order is stable across restarts
- **WHEN** the catalog is read twice in separate runs of the host, with no change to any declaration
- **THEN** both reads return the sources in the same order

#### Scenario: Disabled source is absent from the catalog
- **WHEN** a declared source is disabled
- **THEN** it is listed by neither the unfiltered catalog nor a kind-filtered catalog

### Requirement: On-demand enrichment fetch

The system SHALL fetch enrichment items for a library manga from every enabled source that supports the requested kind, resolve the manga's current title, and merge the returned items into persistent storage tagged with the source label of the provider that returned them. Items SHALL be stored per source, so one kind MAY hold items from several sources at once, and an item already stored for that manga and kind SHALL be skipped (no duplicates). When two sources report the same item, the higher-precedence source SHALL be the one recorded. An empty provider result SHALL NOT delete existing rows, and a source that fails or times out SHALL NOT prevent the remaining sources from being stored.

#### Scenario: Fetch categories from a built-in source
- **WHEN** an enrichment fetch for kind `categories` is requested for a library manga and the `mangadex` source declares `categories`
- **THEN** the categories the `mangadex` source returns are persisted with the `mangadex` source label and previously stored categories are unchanged

#### Scenario: Duplicate items are not stored twice
- **WHEN** a fetch returns an item already stored for that manga and kind
- **THEN** no duplicate row is created and the existing row is preserved

#### Scenario: Empty result preserves stored data
- **WHEN** a provider returns no items
- **THEN** the existing stored items for that manga and kind remain unchanged

#### Scenario: Unknown source or kind
- **WHEN** a fetch is requested for a source that is not in the catalog or does not support the requested kind
- **THEN** the system responds with an error indicating the source/kind is unavailable and stores nothing

#### Scenario: Fetch gathers every enabled source
- **WHEN** an enrichment fetch for kind `categories` is requested for a library manga and both `mangadex` and `kitsu` declare `categories` and are enabled
- **THEN** the categories from both sources are persisted, each carrying its own source label

#### Scenario: Failing source does not block the rest
- **WHEN** one enabled source errors or times out during a fetch
- **THEN** the items returned by the remaining enabled sources are still stored

### Requirement: Category enrichment

The system SHALL store fetched categories (genres/tags) per manga with their source label, deduplicated per manga, and SHALL include the stored categories in the manga detail view data. A fetched category SHALL be resolved through the host's genre alias map before storage, so spellings the map treats as the same genre are stored once under the canonical name; a category the map does not know SHALL be kept exactly as the source sent it.

#### Scenario: Categories rendered on detail view
- **WHEN** the manga detail view is requested for a manga with stored categories
- **THEN** the categories and their source labels are included in the response

#### Scenario: Same genre from two sources is stored once
- **WHEN** two sources report the same genre under spellings the alias map treats as one
- **THEN** a single category is stored under the canonical name

#### Scenario: Unknown category is kept as received
- **WHEN** a source reports a category the alias map does not know
- **THEN** it is stored with the spelling the source sent

#### Scenario: Category toggle uses the stored spelling
- **WHEN** the user toggles a stored category into the manga's genres
- **THEN** the stored genre override uses the same spelling as the stored category

### Requirement: Author enrichment

The system SHALL support `authors` as an enrichment kind, and SHALL store a fetched author on the manga rather than as a list of alternative rows. Because a manga holds a single author value, only the author reported by the highest-precedence source that returned a non-blank author SHALL be stored; authors reported by the other sources SHALL NOT be joined into that value. A fetched author SHALL replace the stored value, blank author names in a result SHALL be ignored, and the manga detail view SHALL use the stored author when the source plugin supplies none.

#### Scenario: Author stored on the manga
- **WHEN** an enrichment fetch for kind `authors` returns author names for a library manga
- **THEN** the manga's author is set to those names and a later fetch replaces the stored value

#### Scenario: Stored author fills a gap
- **WHEN** the detail view is requested for a manga whose source plugin reported no author and a stored author exists
- **THEN** the view shows the stored author

#### Scenario: Source plugin author wins
- **WHEN** the source plugin supplies an author while a stored author also exists
- **THEN** the view shows the plugin's author and the stored value is left untouched

#### Scenario: Highest precedence source owns the author
- **WHEN** several enabled sources each report an author for the same manga
- **THEN** only the author from the highest-precedence source is stored

## REMOVED Requirements

### Requirement: Compact enrichment fetch panel

**Reason**: The requirement demanded a kind selector and a source selector that the detail view never rendered, and a fetch that gathers every enabled source makes both selections meaningless; keeping it would force selector UI whose only effect is to narrow a result the user wants widened.

**Migration**: The enrichment control becomes a single fetch action, specified by "Enrichment fetch control". Nothing is lost for users, because no selector was ever rendered and so no kind or source selection was ever made there.

## ADDED Requirements

### Requirement: Enrichment fetch control

The manga detail view SHALL present a single enrichment fetch action. The control SHALL be collapsed by default and expandable by the user. Because the action fetches every kind from every enabled source, the control SHALL NOT require the user to select a kind or a source before submitting.

#### Scenario: Control collapsed by default
- **WHEN** the detail view first renders
- **THEN** the enrichment control is collapsed and the detail content is shown without the fetch form expanded

#### Scenario: Submit needs no kind or source selection
- **WHEN** the user submits the enrichment fetch action
- **THEN** every enabled source that supports at least one kind is queried and the user was required to select nothing beyond submitting

#### Scenario: Result items appear under their source
- **WHEN** a submitted fetch stores items from more than one source
- **THEN** the detail sections show the stored items each labelled with the source that provided it

### Requirement: Enrichment source precedence

A source declaration MAY carry an integer precedence and an enabled flag. A lower precedence value SHALL run before a higher one. When a declaration omits the precedence, the source SHALL be treated as the least preceding; when it omits the enabled flag, the source SHALL be treated as enabled. Precedence SHALL be resolved by the host from the declarations, not from the order in which scripts are discovered or loaded, so the resulting order SHALL be independent of map iteration and SHALL be identical across restarts. A source that is disabled SHALL be neither fetched nor offered.

#### Scenario: Declared precedence orders the sources
- **WHEN** two sources declare `titles` with precedence 1 and 1000
- **THEN** the precedence 1 source precedes the precedence 1000 source

#### Scenario: Undeclared precedence falls last
- **WHEN** one source declares a precedence and another declares none
- **THEN** the undeclared source follows the declared one

#### Scenario: Order does not depend on load order
- **WHEN** the same set of declarations is loaded in different orders
- **THEN** the resulting precedence order is the same

#### Scenario: Disabled source is never fetched
- **WHEN** a source is disabled and a fetch runs
- **THEN** that source is not queried

### Requirement: Fetched records match the requested title

An enrichment source SHALL return only records that belong to the title it was asked about. A candidate record SHALL be accepted only when the searched title matches the candidate's own title or one of its alternative titles after normalization, where normalization ignores case, punctuation, and spacing. A candidate that matches nothing SHALL be discarded rather than returned. When no candidate matches, the source SHALL return no items instead of the closest result it happened to receive.

#### Scenario: Candidate matching the searched title is accepted
- **WHEN** a search for a title returns a record whose own title matches it after normalization
- **THEN** the record's data is returned for that title

#### Scenario: Candidate matching only an alternative title is accepted
- **WHEN** a search for a title returns a record whose own title differs but whose alternative titles include a normalized match
- **THEN** the record's data is returned for that title

#### Scenario: Unrelated candidate is discarded
- **WHEN** a search returns only records whose titles and alternative titles match nothing after normalization
- **THEN** no items are returned for that title

#### Scenario: Search punctuation and casing do not defeat the match
- **WHEN** the searched title differs from the record's title only by case, punctuation, or spacing
- **THEN** the record is still accepted

### Requirement: Summary language and hygiene

A fetched summary SHALL be written in one of the languages the declaring source prefers, or the source SHALL return no summary at all. A source SHALL NOT fall back to a summary in a language it was not asked for. A returned summary SHALL be prose: it SHALL NOT be truncated, and it SHALL NOT contain a list of bare URLs, promotional link blocks, or trailer links.

#### Scenario: Preferred language is used
- **WHEN** a record offers a summary in a language the source prefers and also in others
- **THEN** the preferred-language summary is returned

#### Scenario: Only unrequested languages are offered
- **WHEN** a record offers a summary only in languages the source does not prefer
- **THEN** no summary is returned for that record

#### Scenario: Summary carries no link spam
- **WHEN** a record's summary includes a list of bare URLs or trailer links
- **THEN** the stored summary contains the prose without that block

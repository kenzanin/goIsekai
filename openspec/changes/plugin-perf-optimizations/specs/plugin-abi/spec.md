## MODIFIED Requirements

### Requirement: Plugin enrichment provider contract

A plugin SHALL be able to declare enrichment providers in its metadata as a list of entries, each containing a source identifier, a display name, and one or more supported enrichment kinds (`titles`, `summaries`, `categories`, `related`). A plugin that declares at least one enrichment provider SHALL export a `GetEnrichment` function accepting a JSON object `{"title": string, "kind": string, "source": string}` and returning a JSON object `{"source": string, "kind": string, "items": [...]}`. The `source` value SHALL be the provider-defined display label used for stored items. The function SHALL be optional: a plugin without it remains a fully functional source plugin. The host SHALL treat an absent, malformed, or erroring response as a failed fetch for that source only.

#### Scenario: Plugin declares a custom enrichment source
- **WHEN** a plugin's metadata contains an enrichment provider entry with source `xxxx` and kind `titles`, and exports `GetEnrichment`
- **THEN** the host includes source `xxxx` in the enrichment catalog for `titles`

#### Scenario: GetEnrichment call shape
- **WHEN** the host invokes `GetEnrichment` with `{"title": "Solo Leveling", "kind": "categories", "source": "xxxx"}`
- **THEN** the plugin returns `{"source": "xxxx", "kind": "categories", "items": [...]}` with at minimum `source` and a possibly-empty `items` array

#### Scenario: Plugin without enrichment capability
- **WHEN** a plugin omits the enrichment provider metadata and does not export `GetEnrichment`
- **THEN** the plugin loads successfully and contributes no enrichment catalog entries

#### Scenario: Malformed enrichment response
- **WHEN** a plugin's `GetEnrichment` returns malformed JSON or an error
- **THEN** the host surfaces a failed fetch for that source and leaves other sources unaffected

### Requirement: Batch detail+chapters plugin function

The plugin ABI SHALL include an optional `GetMangaDetailWithChapters(mangaID string) (string, error)` function. When exported, this function SHALL return a JSON object containing both manga detail and chapter list in a single response: `{"detail": <Manga>, "chapters": [<Chapter>]}`. The host SHALL invoke this function instead of separate `GetMangaDetail` + `GetChapterList` calls when both results are needed for the same manga. Plugins that do not export this function SHALL continue to work via separate calls.

#### Scenario: Plugin implements batch function
- **WHEN** a plugin exports `GetMangaDetailWithChapters`
- **THEN** the host invokes it instead of separate `GetMangaDetail` + `GetChapterList` calls when both are needed

#### Scenario: Plugin does not implement batch function
- **WHEN** a plugin does not export `GetMangaDetailWithChapters`
- **THEN** the host falls back to separate `GetMangaDetail` + `GetChapterList` calls

#### Scenario: Batch function returns valid response
- **WHEN** `GetMangaDetailWithChapters` is invoked and returns valid JSON with `detail` and `chapters`
- **THEN** the host parses both and returns them as separate typed results

#### Scenario: Batch function returns error
- **WHEN** `GetMangaDetailWithChapters` returns an error
- **THEN** the host returns the error without falling back to separate calls

#### Scenario: Legacy plugin unaffected
- **WHEN** a plugin does not export `GetMangaDetailWithChapters`
- **THEN** the host uses separate `GetMangaDetail` + `GetChapterList` calls as before

## ADDED Requirements

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

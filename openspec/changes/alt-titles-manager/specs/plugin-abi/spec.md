# Plugin ABI Specification (delta)

## ADDED Requirements

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
- **THEN** the plugin returns `{"source": "MangaDex", "titles": ["...", ...}` with at minimum `source` and a possibly-empty `titles` array

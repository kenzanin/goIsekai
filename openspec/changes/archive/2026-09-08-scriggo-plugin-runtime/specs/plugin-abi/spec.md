## ADDED Requirements

### Requirement: Scriggo runtime kind

The plugin manager SHALL accept `"scriggo"` as a valid plugin runtime kind alongside `"wasm"`, `"lua"`, `"js"`, and `"go"`. Scriggo plugins SHALL implement the same ABI function contract (same function names, same JSON argument/return shapes) as all other runtime kinds.

#### Scenario: Scriggo plugin implements full ABI

- **WHEN** a Scriggo plugin exports `Init`, `SearchManga`, `GetMangaDetails`, `GetChapterList`, and `GetPageList`
- **THEN** the host accepts it as a valid plugin and dispatches ABI calls identically to other runtime kinds

#### Scenario: Scriggo plugin with alt-titles capability

- **WHEN** a Scriggo plugin also exports `GetAltTitles`
- **THEN** the host exposes its alt-title servers through the same capability-discovery mechanism as other runtimes

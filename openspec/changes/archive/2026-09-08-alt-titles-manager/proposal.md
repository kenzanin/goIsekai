# Proposal: alt-titles-manager

## Why

Library titles come from each source plugin and are often localized or inconsistent across sources. Users need to (a) resolve a manga's alternative titles from a chosen lookup server, (b) promote any alternative title to the manga's main library title, and (c) curate the list — so that duplicate detection and library search work reliably across 2000+ titles.

## What Changes

- **Alt-titles enricher plugin contract**: plugins may declare `alt_title_servers` (array of `{id, name}`) in their metadata and expose an optional `getAltTitles({"title", "server"})` ABI function. All knowledge of upstream APIs (MangaDex, MAL, AI, …) stays inside plugins; the host never hardcodes a provider.
- **Storage**: new `alt_titles` table (`manga_row_id`, `title`, `source`), replacing the stopgap `mangas.alt_titles` JSON TEXT column (dropped; only one row currently uses it). Unique per (manga, title); providers merge with skip-duplicate semantics.
- **API (api-first — every UI feature has a JSON endpoint)**:
  - `GET /api/alt-title-servers` — aggregate `{providerPlugin, serverId, name}` from all plugins declaring the capability.
  - `POST /api/manga/{pluginID}/{mangaID}/alt-titles` `{"server"}` — fetch from provider plugin, merge into table.
  - `DELETE /api/manga/{pluginID}/{mangaID}/alt-titles` `{"title"}` — remove one title row.
  - `PUT /api/manga/{pluginID}/{mangaID}/title` `{"title"}` — set main title; must exist in alt_titles; old main title is inserted back into the list.
  - `GET /api/library/search?q=` — fuzzy library search over title + alt titles (FTS5 candidate lookup + Go-side scoring).
- **FTS5 search index**: `library_fts` virtual table over main + alt titles, kept in sync on write, with Go-side typo-tolerant scoring on top.
- **UI**: detail page — provider/server dropdown + fetch button; chips per alt title with `via <source>` badge; click chip → set as main title; × on chip → remove. Library page — search box filtering the grid.

## Capabilities

### New Capabilities
- `alt-titles`: plugin enricher contract, storage, and user curation of alternative titles (set/remove/fetch-by-server).

### Modified Capabilities
- `plugin-abi`: adds the optional `GetAltTitles` function and `alt_title_servers` metadata to the plugin contract.
- `http-api`: adds the five new JSON endpoints above.
- `storage`: adds the `alt_titles` table, drops `mangas.alt_titles`, adds the `library_fts` FTS5 index with sync.

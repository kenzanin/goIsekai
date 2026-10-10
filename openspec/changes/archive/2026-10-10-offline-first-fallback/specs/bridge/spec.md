# Delta: bridge

## ADDED Requirements

### Requirement: Manga detail page renders from persisted data when the plugin is unreachable

When the live plugin fetch for a manga fails with an error other than an
anti-bot challenge, the detail page SHALL retry with the manga's persisted copy
(`CachedMangaAndChapters` plus the enrichment-derived fields the live path
fills: stored author, override genres, cover dimensions, alt titles, alt
summaries, categories, related entries). If the persisted copy exists, the page
SHALL render normally with a visible cached-data notice. If neither the plugin
nor a persisted copy exists, the server SHALL return the existing error
response.

#### Scenario: Detail page opens while offline

- **WHEN** the plugin for the requested manga cannot be reached
- **AND** the manga has a persisted copy in the database
- **THEN** the detail page renders with the persisted title, cover, description,
  status, author, genres, enrichment rows, and chapter list
- **AND** the page shows a cached-data notice

#### Scenario: Detail page for an unknown manga while offline

- **WHEN** the plugin cannot be reached
- **AND** the manga has no persisted copy
- **THEN** the server responds with the existing error page

#### Scenario: Online loads show no notice

- **WHEN** the detail page loads via a successful plugin fetch
- **THEN** no cached-data notice is rendered

### Requirement: Plugin response caching

The plugin manager SHALL be wired to the application database at startup, and
plugin responses for `GetMangaDetail` and `GetChapterList` SHALL be stored in
`plugin_cache` and served from it while unexpired. `GetMangaDetail` responses
SHALL use the `cache_ttl_hours` value (default 24); `GetChapterList` responses
SHALL use the `chapter_cache_ttl_hours` value (default 168).

#### Scenario: Second detail page load serves from cache

- **WHEN** a manga's detail page is loaded twice within the TTL
- **THEN** the second load does not invoke the plugin for `GetMangaDetail`
- **AND** the response equals the first load's data

#### Scenario: Chapter list survives past 24 hours

- **WHEN** a chapter list was cached 25 hours ago
- **AND** `chapter_cache_ttl_hours` is 168
- **THEN** the next chapter list request is served from `plugin_cache`

# Plugin Caching Specification

## Purpose

Cache plugin HTTP responses in the database so repeated requests for manga already in the library skip network calls entirely, serving instantly from local storage.

## Requirements

### Requirement: Cache plugin detail responses
The system SHALL cache plugin `GetMangaDetail` responses in the database after the first successful fetch. Subsequent requests for the same manga SHALL return the cached response without invoking the plugin or making HTTP requests.

#### Scenario: First fetch populates cache
- **WHEN** a manga detail is requested and no cache entry exists
- **THEN** the system invokes the plugin, stores the response in the cache, and returns the result

#### Scenario: Subsequent fetch serves from cache
- **WHEN** a manga detail is requested and a valid cache entry exists
- **THEN** the system returns the cached response without invoking the plugin

#### Scenario: Cache miss triggers fresh fetch
- **WHEN** a manga detail is requested with no cache entry
- **THEN** the system invokes the plugin and stores the result

### Requirement: Cache plugin chapter list responses
The system SHALL cache plugin `GetChapterList` responses in the database. Cached chapter lists SHALL be returned without invoking the plugin when a valid cache entry exists.

#### Scenario: Chapter list cached after first fetch
- **WHEN** chapter list is requested and no cache entry exists
- **THEN** the system invokes the plugin, caches the chapter list, and returns it

#### Scenario: Cached chapter list served directly
- **WHEN** chapter list is requested with a valid cache entry
- **THEN** the system returns the cached chapters without plugin invocation

### Requirement: Manual cache refresh
The system SHALL provide a mechanism to force-refresh cached data by re-invoking the plugin and updating the cache entry.

#### Scenario: User triggers refresh
- **WHEN** a user requests a refresh for cached manga data
- **THEN** the system invokes the plugin, replaces the cache entry, and returns the fresh result

#### Scenario: Refresh clears stale cache
- **WHEN** a refresh is triggered for manga with an existing cache entry
- **THEN** the old cache entry is replaced with the new response

### Requirement: Cache staleness threshold
The system SHALL treat cache entries older than a configurable threshold as stale. Stale entries SHALL trigger a background refresh while serving the stale data immediately.

#### Scenario: Stale cache triggers background refresh
- **WHEN** a cached entry exceeds the staleness threshold
- **THEN** the system serves the stale data immediately and invokes the plugin in the background to update the cache

#### Scenario: Fresh cache served directly
- **WHEN** a cached entry is within the staleness threshold
- **THEN** the system serves the cached data without any background refresh

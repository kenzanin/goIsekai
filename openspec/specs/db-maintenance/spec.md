# Database Maintenance Specification

## Purpose

Keeps the SQLite library small and cheap to query over time by discarding data the application can re-derive or no longer needs: chapter metadata of manga that were never added to the library, superseded read-history detail, and cached page lists for already-read chapters.

## Requirements

### Requirement: Non-library chapter purging
The system SHALL NOT keep chapter rows for manga whose `in_library` flag is false. The purge SHALL run as part of routine maintenance, and the detail-fetch path SHALL NOT persist chapters for manga that are not in the library at fetch time. Chapters for such manga SHALL be fetched again on demand when the manga is opened.

#### Scenario: Viewed from search but never added
- **WHEN** a manga detail page is fetched from search results without adding the manga to the library
- **THEN** no chapter rows are persisted for it, and opening the chapter list later fetches and displays chapters from the plugin

#### Scenario: Maintenance removes leftover non-library chapters
- **WHEN** maintenance runs and chapter rows exist for manga with `in_library=0` (e.g. from a removal or a legacy database)
- **THEN** those chapter rows and their dependents are deleted

#### Scenario: Removing from library
- **WHEN** a manga is toggled out of the library
- **THEN** its chapter rows and their dependents are deleted by the next maintenance run, and adding it back re-fetches chapters on demand

### Requirement: Read history deduplication
The read history SHALL keep at most one row per chapter, recording the latest read event (upsert on read). Aggregations the reading-history page relies on (most recent read per manga, chapters read) SHALL be unaffected.

#### Scenario: Re-reading a chapter
- **WHEN** the reader advances through pages of a chapter that already has a read-history row
- **THEN** that row's page number and timestamp are updated instead of inserting new rows

#### Scenario: History page shows latest activity
- **WHEN** the reading-history page is rendered
- **THEN** each manga appears with its most recent read time as before deduplication

### Requirement: Page-list cache pruning
Stored chapter page lists SHALL be treated as a cache: rows for chapters that are fully read MAY be deleted by maintenance, and deleting a page-list row SHALL never break reading — the next open re-fetches the list from the plugin.

#### Scenario: Fully read chapter cache eviction
- **WHEN** maintenance runs and a page-list row exists for a chapter with `is_read=1`
- **THEN** the row is deleted and re-opening that chapter fetches and re-caches the page list

#### Scenario: Unread chapters keep their cache
- **WHEN** maintenance runs and a page-list row exists for a chapter that is not fully read
- **THEN** the row is kept

### Requirement: Post-migration compaction
After a migration that deletes a significant amount of data, the system SHALL `VACUUM` the database file so freed pages are returned to the filesystem.

#### Scenario: File shrinks after the purge migration
- **WHEN** the compaction migration completes on a database that carried non-library chapters and duplicate read-history rows
- **THEN** the database file size on disk reflects only live data

### Requirement: Image cache size limit

The image cache directory (`<cache_dir>/images`) SHALL be pruned to a configured
maximum size. When the total size of cached image files exceeds `max_cache_gb`
(default 2) after an image download or at a maintenance tick, files SHALL be
deleted until the total is at or below the limit, and the freed amount SHALL be
logged. Files SHALL be deleted in order of least recent use, where serving a
file from the cache counts as use. Files that belong to a library manga (its
cover and the pages of its chapters) SHALL be deleted only after every other
file has been deleted. A `max_cache_gb` of 0 SHALL disable size-based pruning
entirely.

#### Scenario: Cache exceeds the configured limit

- **WHEN** the image cache totals more than `max_cache_gb`
- **AND** maintenance runs or an image download completes
- **THEN** the least recently used files are deleted until the cache totals at
  most the limit
- **AND** the freed amount is logged

#### Scenario: Recently written files survive

- **WHEN** pruning deletes overflow files
- **THEN** files written most recently (the chapter currently being read) remain

#### Scenario: Pruning disabled

- **WHEN** `max_cache_gb` is 0
- **THEN** no files are ever deleted by the size limit

#### Scenario: Reading a cached file keeps it

- **WHEN** a cached image is served from the disk cache
- **THEN** it ranks as recently used, so it is deleted after files that were
  written later but never read

#### Scenario: Library files are deleted last

- **WHEN** the cache exceeds the limit and holds both library and non-library files
- **THEN** non-library files are deleted first, whatever their age
- **AND** a library manga's cover and chapter pages remain while any non-library file remains

#### Scenario: Library alone exceeds the limit

- **WHEN** only library files remain and the cache still exceeds the limit
- **THEN** the least recently used library files are deleted until the cache is at most the limit

### Requirement: Changed cache limit is applied without a restart
When the hot-reloaded `max_cache_gb` is lower than the previous value and the image cache currently exceeds the new limit, the host SHALL prune the cache down to the new limit without waiting for a restart or the next maintenance tick. The prune SHALL run off the request that saved the setting and SHALL log the freed amount. Raising the limit, or a reload that does not change it, SHALL NOT delete any file. A limit of 0 SHALL delete nothing.

#### Scenario: Lowering the limit prunes
- **WHEN** the cache totals 3 GB and the user lowers `max_cache_gb` to 2
- **THEN** the cache is pruned to at most 2 GB shortly after the config reload
- **AND** the freed amount is logged

#### Scenario: Raising the limit deletes nothing
- **WHEN** the user raises `max_cache_gb`
- **THEN** no cached file is deleted

#### Scenario: Reload without a change
- **WHEN** the config file is reloaded and `max_cache_gb` is the same as before
- **THEN** no prune is triggered by the reload

#### Scenario: Limit of zero
- **WHEN** the user sets `max_cache_gb` to 0
- **THEN** no file is deleted

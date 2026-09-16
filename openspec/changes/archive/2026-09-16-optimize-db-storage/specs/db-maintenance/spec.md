## Purpose

Keeps the SQLite library small and cheap to query over time by discarding data the application can re-derive or no longer needs: chapter metadata of manga that were never added to the library, superseded read-history detail, and cached page lists for already-read chapters.

## ADDED Requirements

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

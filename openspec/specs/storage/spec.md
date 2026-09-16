# Storage Specification

## Purpose

Persists the reader's library bookmarks, chapter metadata and read progress, download state, read history, and installed plugins in a Pure-Go SQLite database so state survives restarts.

## Requirements

### Requirement: Manga library persistence
The system SHALL store manga in a `mangas` table with an integer primary key `id` (auto-increment), `plugin_id`, `source_manga_id`, `title`, `cover_url`, optional `description` and `status`, an `in_library` flag, and a unique constraint on `(plugin_id, source_manga_id)`.

#### Scenario: Store manga from a source
- **WHEN** the host saves a manga fetched from a plugin
- **THEN** it persists `plugin_id`, `source_manga_id`, and `title` such that re-importing the same source manga does not create a duplicate row

### Requirement: Chapter persistence
The system SHALL store chapters in a `chapters` table with an integer surrogate `id` primary key, an integer `manga_id` referencing the manga row, `source_chapter_id`, `title`, numeric `chapter_num`, optional `volume_num`, `is_read`, `last_page_read`, and a `download_status` field defaulting to `NOT_DOWNLOADED`. A unique constraint SHALL hold on `(manga_id, source_chapter_id)` and an index SHALL exist on `manga_id` so chapter lists are index lookups rather than scans.

#### Scenario: Record chapter read progress
- **WHEN** the reader advances to a page within a chapter
- **THEN** the chapter's `last_page_read` and `is_read` state persist across restarts

#### Scenario: Chapter list lookup uses the index
- **WHEN** the chapter list of a manga is queried
- **THEN** the rows are found via the `manga_id` index without scanning other manga's chapters

#### Scenario: Re-syncing does not duplicate chapters
- **WHEN** the same chapter is persisted twice for the same manga
- **THEN** the unique constraint on `(manga_id, source_chapter_id)` updates the existing row instead of inserting a duplicate

### Requirement: Download status tracking
The system SHALL track a chapter's download lifecycle using `download_status` values `NOT_DOWNLOADED`, `DOWNLOADING`, and `DOWNLOADED`.

#### Scenario: Download state transitions
- **WHEN** a chapter download begins, progresses, and completes
- **THEN** its `download_status` transitions through `DOWNLOADING` and ends at `DOWNLOADED`

### Requirement: Read history persistence
The system SHALL store read history in a `read_history` table referencing the chapter's integer key with cascade delete, keeping at most one row per chapter that is updated (page number, timestamp) on each read event.

#### Scenario: Record a read event
- **WHEN** the reader opens a page
- **THEN** the chapter's read-history row reflects the page number and read timestamp of the latest event

#### Scenario: History does not grow per page turn
- **WHEN** the reader turns through many pages of one chapter
- **THEN** the read-history table holds a single row for that chapter

### Requirement: Plugin registry persistence
The system SHALL store installed plugins in a `plugins` table with `id`, `name`, `version`, `wasm_path`, `is_active`, and optional `icon_url`, `thumb_ratio`, `cover_dim`, and `author`.

#### Scenario: Register an installed plugin
- **WHEN** a plugin is installed from a `.wasm` file
- **THEN** its metadata (name, version, wasm path, active flag) is recorded and survives restarts

### Requirement: Cascading cleanup
Deleting a manga SHALL cascade-delete its chapters, and deleting a chapter SHALL cascade-delete its read history and chapter pages.

#### Scenario: Remove manga from library
- **WHEN** a manga is removed
- **THEN** its chapters and their read history rows are removed without orphaned records

### Requirement: Accurate persistence of progress and downloads
Manga progress, offline chapter downloads, and library bookmarks SHALL persist accurately in SQLite across app restarts.

#### Scenario: State survives restart
- **WHEN** the app is restarted after progress, downloads, and bookmarks changed
- **THEN** all three are restored exactly as last saved

### Requirement: Alternative titles table
The system SHALL store alternative titles in an `alt_titles` table with an integer primary key `id`, `manga_row_id` foreign key referencing `mangas(id)` with `ON DELETE CASCADE`, a `title` text column, and a `source` text column holding the provider-reported badge label, unique on `(manga_row_id, title)`. The stopgap `mangas.alt_titles` JSON text column is dropped in the same migration.

#### Scenario: Cascade on manga delete
- **WHEN** a manga row is deleted
- **THEN** its alternative title rows are deleted automatically

#### Scenario: Same title from two providers
- **WHEN** two providers return the same title string for one manga
- **THEN** only one row exists and the first-stored source label is kept

### Requirement: Library full-text index
The system SHALL maintain an FTS5 virtual table `library_fts` indexing each library manga's main title and stored alternative titles, kept in sync whenever a manga's title, library membership, or alternative titles change.

#### Scenario: Index reflects title promotion
- **WHEN** a manga's main title is swapped with an alternative title
- **THEN** subsequent full-text queries match both the new main title and the old one (now an alternative)

### Requirement: Storage optimization via purge policy
The system SHALL apply a purge policy to reclaim space: chapters of non-library manga are deleted, cached pages of finished chapters are dropped, and duplicate read history rows (keeping the newest per chapter) are removed.

#### Scenario: Prune storage
- **WHEN** the database is migrated or maintenance runs
- **THEN** non-library chapters and their pages are dropped, read history is deduplicated, and the file size shrinks

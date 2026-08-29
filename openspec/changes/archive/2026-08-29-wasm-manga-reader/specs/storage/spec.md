## Purpose

Persists the reader's library bookmarks, chapter metadata and read progress, download state, read history, and installed plugins in a Pure-Go SQLite database so state survives restarts.

## ADDED Requirements

### Requirement: Manga library persistence
The system SHALL store manga in a `mangas` table keyed by `id`, with `plugin_id`, `source_manga_id`, `title`, `cover_url`, optional `description` and `status`, an `in_library` flag, and a unique constraint on `(plugin_id, source_manga_id)`.

#### Scenario: Store manga from a source
- **WHEN** the host saves a manga fetched from a plugin
- **THEN** it persists `plugin_id`, `source_manga_id`, and `title` such that re-importing the same source manga does not create a duplicate row

### Requirement: Chapter persistence
The system SHALL store chapters in a `chapters` table with `id`, `manga_id`, `source_chapter_id`, `title`, numeric `chapter_num`, optional `volume_num`, `is_read`, `last_page_read`, and a `download_status` field defaulting to `NOT_DOWNLOADED`.

#### Scenario: Record chapter read progress
- **WHEN** the reader advances to a page within a chapter
- **THEN** the chapter's `last_page_read` and `is_read` state persist across restarts

### Requirement: Download status tracking
The system SHALL track a chapter's download lifecycle using `download_status` values `NOT_DOWNLOADED`, `DOWNLOADING`, and `DOWNLOADED`.

#### Scenario: Download state transitions
- **WHEN** a chapter download begins, progresses, and completes
- **THEN** its `download_status` transitions through `DOWNLOADING` and ends at `DOWNLOADED`

### Requirement: Read history persistence
The system SHALL store per-page read events in a `read_history` table referencing the chapter with cascade delete.

#### Scenario: Record a read event
- **WHEN** the reader opens a page
- **THEN** a `read_history` row is written recording the chapter id, page number, and read timestamp

### Requirement: Plugin registry persistence
The system SHALL store installed plugins in a `plugins` table with `id`, `name`, `version`, `wasm_path`, `is_active`, and optional `icon_url`.

#### Scenario: Register an installed plugin
- **WHEN** a plugin is installed from a `.wasm` file
- **THEN** its metadata (name, version, wasm path, active flag) is recorded and survives restarts

### Requirement: Cascading cleanup
Deleting a manga SHALL cascade-delete its chapters, and deleting a chapter SHALL cascade-delete its read history.

#### Scenario: Remove manga from library
- **WHEN** a manga is removed
- **THEN** its chapters and their read history rows are removed without orphaned records

### Requirement: Accurate persistence of progress and downloads
Manga progress, offline chapter downloads, and library bookmarks SHALL persist accurately in SQLite across app restarts.

#### Scenario: State survives restart
- **WHEN** the app is restarted after progress, downloads, and bookmarks changed
- **THEN** all three are restored exactly as last saved

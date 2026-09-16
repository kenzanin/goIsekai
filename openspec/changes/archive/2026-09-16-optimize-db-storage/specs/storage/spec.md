## MODIFIED Requirements

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

### Requirement: Read history persistence
The system SHALL store read history in a `read_history` table referencing the chapter's integer key with cascade delete, keeping at most one row per chapter that is updated (page number, timestamp) on each read event.

#### Scenario: Record a read event
- **WHEN** the reader opens a page
- **THEN** the chapter's read-history row reflects the page number and read timestamp of the latest event

#### Scenario: History does not grow per page turn
- **WHEN** the reader turns through many pages of one chapter
- **THEN** the read-history table holds a single row for that chapter

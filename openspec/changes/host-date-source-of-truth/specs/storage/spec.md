# Storage Specification (Delta)

## Purpose

Chapter persistence retains the moment a chapter was released, and the source's own wording when no moment could be read.

## MODIFIED Requirements

### Requirement: Chapter persistence

The system SHALL store chapters in a `chapters` table with an integer surrogate `id` primary key, an integer `manga_id` referencing the manga row, `source_chapter_id`, `title`, numeric `chapter_num`, optional `volume_num`, `is_read`, `last_page_read`, and a `download_status` field defaulting to `NOT_DOWNLOADED`. A unique constraint SHALL hold on `(manga_id, source_chapter_id)` and an index SHALL exist on `manga_id` so chapter lists are index lookups rather than scans. The table SHALL also carry `released_text` for the source's own release wording when the source supplied no usable moment, so the chapter row can show that wording verbatim; `released_at` SHALL remain NULL in that case, and both SHALL be empty for a source that reports nothing.

#### Scenario: Record chapter read progress

- **WHEN** the reader advances to a page within a chapter
- **THEN** the chapter's `last_page_read` and `is_read` state persist across restarts

#### Scenario: Chapter list lookup uses the index

- **WHEN** the chapter list of a manga is queried
- **THEN** the rows are found via the `manga_id` index without scanning other manga's chapters

#### Scenario: Re-syncing does not duplicate chapters

- **WHEN** the same chapter is persisted twice for the same manga
- **THEN** the unique constraint on `(manga_id, source_chapter_id)` updates the existing row instead of inserting a duplicate

#### Scenario: Release wording survives persistence

- **WHEN** a chapter's source reports a release value the host cannot read as a moment
- **THEN** the wording is stored in `released_text` and `released_at` stays NULL

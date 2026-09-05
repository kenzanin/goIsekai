# Storage Specification (delta)

## ADDED Requirements

### Requirement: Alternative titles table
The system SHALL store alternative titles in an `alt_titles` table keyed by row id, with `manga_row_id` referencing `mangas(id)` with `ON DELETE CASCADE`, a `title` text column, and a `source` text column holding the provider-reported badge label, unique on `(manga_row_id, title)`. The stopgap `mangas.alt_titles` JSON text column SHALL be dropped in the same migration.

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

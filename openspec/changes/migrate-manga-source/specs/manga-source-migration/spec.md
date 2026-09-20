## Purpose

Lets a reader move a title that is already in the library to a different source
plugin, so a title survives a source dying or degrading without losing the
reader's place in it.

## ADDED Requirements

### Requirement: Migration candidates come from the active sources

For a library entry, the system SHALL search every active source plugin for the
same title, using the entry's current display title as the search term. Plugins
that only provide metadata SHALL be excluded, and the entry's current source
SHALL be excluded. When the search on the display title yields no candidate, the
system MAY retry using the entry's alternative titles.

#### Scenario: Candidate found on the display title
- **WHEN** the reader asks to migrate a library entry and another active source returns an exact title match
- **THEN** that source is offered as a migration candidate

#### Scenario: Metadata-only plugins are not candidates
- **WHEN** a plugin provides metadata providers only and no manga search
- **THEN** it is never offered as a migration candidate

#### Scenario: Current source is not a candidate
- **WHEN** candidates are collected for a library entry
- **THEN** the entry's own current source is excluded from the candidate list

#### Scenario: No candidate on the display title
- **WHEN** the display title search returns no candidate on any other source
- **THEN** the system retries the search using the entry's alternative titles

### Requirement: Automatic candidate selection only on an unambiguous match

The system SHALL select a target source automatically when exactly one candidate's
title equals the entry's display title after normalization. When no candidate
matches exactly, or when more than one does, the system SHALL present the
candidates and require the reader to choose rather than picking one itself.

#### Scenario: Single exact match is selected
- **WHEN** exactly one candidate's normalized title equals the entry's normalized display title
- **THEN** that candidate is selected as the migration target without further input

#### Scenario: No exact match lists the candidates
- **WHEN** no candidate's normalized title equals the entry's normalized display title
- **THEN** the candidates are listed for the reader to choose from

#### Scenario: Several exact matches list the candidates
- **WHEN** more than one candidate's normalized title equals the entry's normalized display title
- **THEN** those candidates are listed for the reader to choose from

### Requirement: Migration repoints the library entry instead of duplicating it

Applying a migration SHALL repoint the existing library entry at the target
source. The entry's alternative titles, categories, related entries and library
search index SHALL remain attached to it, and the title MUST NOT appear as a
second entry in the library as a result of migrating.

#### Scenario: Enrichment stays attached
- **WHEN** a library entry with alternative titles and categories is migrated
- **THEN** the migrated entry still has those alternative titles and categories

#### Scenario: Title is not duplicated
- **WHEN** a migration completes
- **THEN** the library still contains exactly one entry for that title

#### Scenario: Title remains findable after migrating
- **WHEN** the reader searches the library for a migrated title
- **THEN** the migrated entry is returned

### Requirement: Read state carries over by chapter number

A migration SHALL mark every chapter of the target source read when its
`chapter_num` is at or below the highest `chapter_num` that was marked read on
the previous source. Target chapters above that boundary SHALL remain unread.
Other per-chapter reading state SHALL NOT be carried over.

#### Scenario: Chapters up to the read boundary become read
- **WHEN** the previous source had chapters read through number 40 and the target source has chapters 1 to 60
- **THEN** target chapters 1 through 40 are marked read

#### Scenario: Chapters beyond the boundary stay unread
- **WHEN** the previous source had chapters read through number 40 and the target source has chapters 1 to 60
- **THEN** target chapters 41 through 60 remain unread

#### Scenario: Nothing read means nothing carried
- **WHEN** no chapter on the previous source was marked read
- **THEN** no chapter on the target source is marked read

### Requirement: The target chapter list replaces the previous one

After a successful migration only the target source's chapters SHALL remain on
the entry. The previous source's chapters SHALL be removed, together with the
page data and read history that belong to them.

#### Scenario: Previous chapters are removed
- **WHEN** a migration to a target source completes
- **THEN** the entry lists only chapters fetched from the target source

### Requirement: Discarded progress is disclosed before the migration is applied

Before applying a migration the system MUST warn the reader that downloaded pages
and the per-chapter reading position on the previous source are discarded and
cannot be recovered, and the migration MUST be applied only in response to an
explicit reader action.

#### Scenario: Warning precedes the action
- **WHEN** the reader selects a migration target
- **THEN** the unrecoverable loss of downloaded pages and per-chapter reading position is stated before the migration can be applied

#### Scenario: Migration never runs on its own
- **WHEN** the reader has not asked for a migration
- **THEN** no migration is performed for any library entry

### Requirement: Migration is refused when the target is already in the library

When the target manga already exists as its own library entry, the system MUST
refuse the migration and MUST report the conflict instead of merging, overwriting
or deleting either entry.

#### Scenario: Conflicting target is reported
- **WHEN** the reader asks to migrate a title to a source that already holds that title as its own library entry
- **THEN** the migration does not run and the conflict is reported to the reader

### Requirement: A failed migration leaves the entry unchanged

When the target source cannot be reached or a migration cannot be completed, the
entry MUST remain on its original source with its original chapters, read state
and enrichment data.

#### Scenario: Target source fails to respond
- **WHEN** the target source returns an error while its chapters are being fetched
- **THEN** the entry is still on its original source with its chapters and read state unchanged

#### Scenario: Migration is interrupted partway
- **WHEN** a migration is interrupted before it completes
- **THEN** the entry is still on its original source with its chapters and read state unchanged

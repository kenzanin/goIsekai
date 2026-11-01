# Plugin ABI Specification (Delta)

## Purpose

Define how a plugin reports a chapter's release date, and what the host does with a value it cannot read as a moment.

## ADDED Requirements

### Requirement: Chapter release date wire shape

The chapter payload SHALL carry `released_at` as unix epoch seconds, or as an RFC 3339 timestamp while plugins are still being migrated. The host SHALL route on the value's JSON type and SHALL NOT guess a layout, resolve a relative phrase, or infer seconds-versus-milliseconds from the digit count.

#### Scenario: Epoch seconds

- **WHEN** a plugin reports `released_at` as the number `1789482933`
- **THEN** the chapter's release date is `2026-09-15T14:35:33Z`

#### Scenario: RFC 3339 while plugins are being migrated

- **WHEN** a plugin reports `released_at` as `2026-09-15T14:35:33Z`
- **THEN** the chapter's release date is `2026-09-15T14:35:33Z`

### Requirement: Unusable release date is retained, not fatal

The host SHALL NOT fail a chapter list because of a release date. A string that is not an RFC 3339 timestamp SHALL be retained verbatim as the source's own wording, and the chapter SHALL decode with no release date. A numeric value outside `1970-01-01T00:00:00Z`..`2100-01-01T00:00:00Z` SHALL be treated as no release date and reported as a warning.

#### Scenario: Prose is retained, not guessed

- **WHEN** a plugin reports `released_at` as `recently`
- **THEN** the chapter decodes with no release date and retains `recently` as its wording

#### Scenario: A millisecond value is not read as seconds

- **WHEN** a plugin reports the epoch-millisecond number `1789482933123`
- **THEN** the chapter carries no release date instead of a year-58600 timestamp

#### Scenario: An unusable value does not break the list

- **WHEN** any chapter in a chapter list reports a release date the host cannot use
- **THEN** the chapter list still returns every chapter

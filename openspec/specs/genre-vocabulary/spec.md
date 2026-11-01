# genre-vocabulary Specification

## Purpose

Define the single canonical genre list the host owns, how a plugin-supplied
genre name is matched to it, and what happens to a name that matches nothing, so
every source lands in the library under one consistent set of names.

## Requirements

### Requirement: Host owns one canonical genre list

The host SHALL define exactly one canonical genre list and SHALL treat it as the
only genre vocabulary. The list SHALL be the 26 MangaDex `genre` group names:
`Action`, `Adventure`, `Boys' Love`, `Comedy`, `Crime`, `Demons`, `Drama`,
`Fantasy`, `Girls' Love`, `Historical`, `Horror`, `Isekai`, `Magical Girls`,
`Martial Arts`, `Mecha`, `Medical`, `Mystery`, `Philosophical`,
`Psychological`, `Romance`, `Sci-Fi`, `Slice of Life`, `Sports`, `Superhero`,
`Thriller`, `Tragedy`, `Wuxia`.

#### Scenario: Vocabulary is fixed at build time

- **WHEN** the host starts with any configuration file
- **THEN** the canonical genre list is the same 26 names, with no entry added to
  or removed from it by that file

#### Scenario: Genre group only

- **WHEN** the canonical list is inspected
- **THEN** it contains no `demographic` group member such as `Shounen`,
  `Seinen` or `Josei`, and no `Ecchi`

### Requirement: Genre matching is case- and separator-insensitive

The host SHALL match a plugin-supplied genre name against the canonical list
case-insensitively and ignoring the separators a plugin may vary on: spaces,
tabs, hyphens, underscores, apostrophes, dots and slashes. A match SHALL return
that canonical spelling.

#### Scenario: Separator variants share one match

- **WHEN** a plugin supplies `sci fi`, `SCI_FI` or `sci-fi`
- **THEN** each resolves to `Sci-Fi`

#### Scenario: Case-only difference matches

- **WHEN** a plugin supplies `romance` or `ROMANCE`
- **THEN** it resolves to `Romance`

#### Scenario: Apostrophe in a canonical name

- **WHEN** a plugin supplies `boys love`
- **THEN** it resolves to `Boys' Love`

### Requirement: Unrecognized genre names are preserved verbatim

The host SHALL keep a genre name that matches no canonical entry exactly as the
plugin supplied it, after trimming surrounding whitespace. The host SHALL NOT
discard, replace or empty such a name.

#### Scenario: Unknown name survives normalization

- **WHEN** a plugin supplies `Ecchi`
- **THEN** the stored and displayed genre is `Ecchi`

#### Scenario: Unknown name is not emptied

- **WHEN** normalization receives a list containing only names outside the
  canonical vocabulary
- **THEN** it returns those names, not an empty list

### Requirement: Genre normalization happens in the host, not via a plugin native

The host SHALL normalize genre names itself wherever plugin-supplied genres
enter the library: the manga row, the enrichment categories, and the per-plugin
genre cache. The host SHALL NOT expose a `normalize_genre` native to plugins.

#### Scenario: Manga genres normalized on read

- **WHEN** a manga whose stored genres include a variant spelling is loaded
- **THEN** the canonical spelling is shown and no new native is required from
  the plugin

#### Scenario: Categories normalized before storage

- **WHEN** enrichment returns categories that match canonical genres
- **THEN** they are stored under the canonical spelling

### Requirement: No config-driven genre dictionary

The host SHALL NOT read a genre alias table, and `goisekai.ini` SHALL NOT carry a
`[genre]` section that extends or overrides the canonical list. An existing
`[genre]` section written by an earlier version SHALL be ignored rather than
treated as an error, and SHALL NOT be written on the next save.

#### Scenario: Old config still loads

- **WHEN** a `goisekai.ini` containing a `[genre]` section is loaded
- **THEN** loading succeeds and the canonical list is unchanged

#### Scenario: Save drops the section

- **WHEN** the configuration is saved after this change
- **THEN** the written file contains no `[genre]` section

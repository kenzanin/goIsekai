## Purpose

Expose shared text utility functions from the host to Lua and JS plugins so plugins stop reimplementing common string transformations and produce consistent, canonical results across sources.

## Requirements

### Requirement: Status normalization native

The host SHALL expose a `normalize_status` text native to both the Lua and JS plugin runtimes. The native SHALL accept exactly one argument, the raw status value, and SHALL return the canonical status label it matches case-insensitively. The canonical vocabulary SHALL be exactly `Ongoing`, `Completed`, `Hiatus`, `Cancelled`, `Upcoming`. When the raw value matches no canonical label the native SHALL return the empty string: the host SHALL NOT emit, echo, or invent a non-canonical status. The native SHALL NOT accept a caller-supplied mapping.

#### Scenario: Known value is normalized
- **WHEN** the native is called with the canonical value `Completed`
- **THEN** it returns `Completed`

#### Scenario: Case-insensitive match
- **WHEN** the raw value differs from a canonical label only in case
- **THEN** it is still matched and normalized

#### Scenario: Unknown value passes through
- **WHEN** the raw value matches no canonical label, such as a site-specific word like `publishing`
- **THEN** it does not pass through: the native returns the empty string

#### Scenario: Available in both runtimes
- **WHEN** a Lua plugin and a JS plugin each call the native with the same raw value
- **THEN** both receive the same result

#### Scenario: Upcoming is part of the vocabulary
- **WHEN** the native is called with `Upcoming`
- **THEN** it returns `Upcoming`

#### Scenario: Cancelled replaces Dropped
- **WHEN** the native is called with `Dropped`
- **THEN** it returns the empty string, and a plugin reporting a discontinued title SHALL send `Cancelled`

#### Scenario: Caller-supplied mapping is not honored
- **WHEN** the native is called with a mapping table as its first argument
- **THEN** it returns the empty string rather than resolving the mapping

### Requirement: Chapter number extraction native

The host SHALL expose a `chapter_num` text native in both the Lua and the JS plugin runtime. The native SHALL accept a raw title, slug or URL string and SHALL return a number: the number attached to a chapter or episode word or to `#` when present, otherwise the first number in the string. The native SHALL return `0` when the string names only a volume, and `0` when it contains no number, so a plugin never derives a chapter number from a volume. A number written with a comma decimal separator SHALL be read as a decimal.

#### Scenario: Chapter word wins over volume word
- **WHEN** the native is called with `Vol. 3 Ch. 12`
- **THEN** it returns `12`

#### Scenario: Bare trailing number
- **WHEN** the native is called with a title that has no chapter word, e.g. `One Piece 1085`
- **THEN** it returns `1085`

#### Scenario: Volume-only string
- **WHEN** the native is called with `Vol. 3`
- **THEN** it returns `0`

#### Scenario: No number present
- **WHEN** the native is called with a string containing no digits
- **THEN** it returns `0`

#### Scenario: Comma decimal separator
- **WHEN** the native is called with `Chapter 12,5`
- **THEN** it returns `12.5`

#### Scenario: Available in both runtimes
- **WHEN** a Lua plugin and a JS plugin each call the native with the same string
- **THEN** both receive the same number

### Requirement: Release date normalization native

The host SHALL expose a `date_to_iso` text native in both the Lua and the JS plugin runtime. The native SHALL accept a raw date string and SHALL return RFC 3339 in UTC. The native SHALL recognize at least unix seconds, unix milliseconds, the relative phrases (`N minutes|hours|days|weeks|months|years ago`, `yesterday`, `today`, `just now`), and the common ISO, `2006-01-02`, `2 Jan 2006` and `Jan 2, 2006` layouts. The native SHALL return an empty string when the input does not parse, so a plugin omits `released_at` rather than inventing a timestamp. Locale-ambiguous slash-only dates SHALL NOT be parsed.

#### Scenario: Unix seconds
- **WHEN** the native is called with `1789482933`
- **THEN** it returns `2026-09-15T14:35:33Z`

#### Scenario: Unix milliseconds
- **WHEN** the native is called with `1789482933123`
- **THEN** it returns `2026-09-15T14:35:33Z`

#### Scenario: Relative phrase
- **WHEN** the native is called with `2 days ago`
- **THEN** it returns a UTC RFC 3339 timestamp two days before now

#### Scenario: Explicit date
- **WHEN** the native is called with `2026-01-02`
- **THEN** it returns `2026-01-02T00:00:00Z`

#### Scenario: Unparseable input
- **WHEN** the native is called with a value it cannot parse, e.g. `next Tuesday`
- **THEN** it returns an empty string

#### Scenario: Available in both runtimes
- **WHEN** a Lua plugin and a JS plugin each call the native with the same string
- **THEN** both receive the same normalized timestamp

### Requirement: Embedded JSON extraction native

The host SHALL expose a `json_blob` text native in both the Lua and the JS plugin runtime. The native SHALL accept the raw page text and an optional marker string, SHALL start its search at the first occurrence of the marker, and SHALL return the first balanced JSON object or array found from there. Braces and brackets inside JSON strings SHALL NOT terminate the blob. The native SHALL return an empty string when the marker is absent or when no balanced blob follows it.

#### Scenario: Blob after a marker
- **WHEN** the native is called with page text containing an `application/ld+json` script holding `{"a":[1,2]}` and that marker
- **THEN** it returns `{"a":[1,2]}`

#### Scenario: Brace inside a string does not close the blob
- **WHEN** the blob contains a string with an unbalanced brace, e.g. `{"t":"a { brace"}`
- **THEN** the native returns the complete blob

#### Scenario: Marker missing
- **WHEN** the marker does not occur in the text
- **THEN** the native returns an empty string

#### Scenario: Available in both runtimes
- **WHEN** a Lua plugin and a JS plugin each call the native with the same arguments
- **THEN** both receive the same blob

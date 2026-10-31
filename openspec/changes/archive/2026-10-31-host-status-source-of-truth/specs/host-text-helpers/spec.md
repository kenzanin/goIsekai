# Spec Delta

## MODIFIED Requirements

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

## REMOVED Requirements

### Requirement: Plugin-overridable default status map

**Reason**: The host must be the single source of truth for the status vocabulary. Publishing a host-owned dictionary of upstream site spellings (`releasing`, `publishing`, `finished`, `on hiatus`, `discontinued`) made the host guess what each site means, and that dictionary had already diverged from the `[status]` configuration copy. A plugin that scrapes a site-specific word is the side responsible for translating it into a canonical label.

**Migration**: Plugins that relied on the host mapping SHALL map their site's wording themselves before returning a status, using the canonical vocabulary `Ongoing`, `Completed`, `Hiatus`, `Cancelled`, `Upcoming`. The `Dropped` label is gone; `cancelled`, `canceled` and `dropped` all map to `Cancelled`. A plugin whose site has no matching canonical state returns no status rather than raw text.

## Purpose

Expose shared text utility functions from the host to Lua and JS plugins so plugins stop reimplementing common string transformations and produce consistent, canonical results across sources.

## ADDED Requirements

### Requirement: Status normalization native

The host SHALL expose a `normalize_status` text native to both the Lua and JS plugin runtimes. The native SHALL accept a mapping table and a raw status string, and SHALL return the canonical status mapped from the raw value using a case-insensitive match on the mapping keys. When the raw value matches no key, the native SHALL return the raw value unchanged. The mapping values SHALL be the canonical vocabulary: `Ongoing`, `Completed`, `Hiatus`, `Dropped`, `Upcoming`.

#### Scenario: Known value is normalized
- **WHEN** the native is called with a mapping that maps `releasing` to `Ongoing` and the raw value `Releasing`
- **THEN** it returns `Ongoing`

#### Scenario: Case-insensitive match
- **WHEN** the raw value differs from the mapping key only in case
- **THEN** it is still matched and normalized

#### Scenario: Unknown value passes through
- **WHEN** the raw value matches no mapping key
- **THEN** the native returns the raw value unchanged

#### Scenario: Available in both runtimes
- **WHEN** a Lua plugin and a JS plugin each call the native with equivalent arguments
- **THEN** both receive the same normalized result

### Requirement: Plugin-overridable default status map

The host SHALL provide a default status mapping accessible to plugins, and plugins SHALL be able to pass their own mapping to the native so that site-specific upstream values (e.g. `on-going`, `publishing`) normalize correctly without a host change. Passing an empty or missing mapping SHALL fall back to the host default mapping.

#### Scenario: Host default map is readable by plugins
- **WHEN** a plugin reads the default status mapping
- **THEN** it receives the host's canonical mapping including at least `releasing`→`Ongoing`, `completed`→`Completed`, and `dropped`→`Dropped`

#### Scenario: Plugin map overrides for site-specific values
- **WHEN** a plugin passes a mapping containing a site-specific key (e.g. `on-going`→`Ongoing`) and the raw value is `ON-GOING`
- **THEN** the native returns `Ongoing`

#### Scenario: Missing map falls back to default
- **WHEN** the native is called with no mapping argument
- **THEN** it normalizes using the host default mapping

## ADDED Requirements

### Requirement: Image priority parameter

The image endpoint (`GET /image`) SHALL accept an optional `prio` query
parameter with value `high` or `low`. The resolved priority SHALL be
passed through to the image fetch pipeline. Absent or unrecognized
values SHALL resolve to `low`.

#### Scenario: Reader marks the on-screen page high priority

- **WHEN** the reader requests the page currently displayed with
  `prio=high`
- **THEN** the fetch pipeline treats that request as high priority

#### Scenario: Reader marks read-ahead pages low priority

- **WHEN** the reader requests read-ahead or neighbor-chapter pages with
  `prio=low` (or without the parameter)
- **THEN** the fetch pipeline treats those requests as low priority

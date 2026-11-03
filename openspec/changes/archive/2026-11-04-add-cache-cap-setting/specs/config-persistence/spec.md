# Spec Delta

## ADDED Requirements

### Requirement: Image cache limit is editable on the Settings page
The Settings page SHALL show the image cache limit in gigabytes next to the current cache size and SHALL save it to `max_cache_gb` through the same Save action as the other settings. The value SHALL be a number greater than or equal to 0, where 0 disables size-based pruning. A submitted value that is not a valid number, or is negative, SHALL leave the stored limit unchanged. A form that does not carry the field SHALL leave the stored limit unchanged.

#### Scenario: Valid limit is saved
- **WHEN** the user enters 5 as the image cache limit and clicks Save
- **THEN** `goisekai.ini` is written with `max_cache_gb` of 5.0
- **AND** the Settings page shows 5 as the limit after the redirect

#### Scenario: Zero disables pruning
- **WHEN** the user enters 0 and clicks Save
- **THEN** `max_cache_gb` is stored as 0.0
- **AND** the page text says size-based pruning is off

#### Scenario: Invalid value is ignored
- **WHEN** the user submits a negative number or text that is not a number
- **THEN** the stored `max_cache_gb` keeps its previous value

#### Scenario: Other settings do not touch the limit
- **WHEN** a save request carries only other settings keys
- **THEN** the stored `max_cache_gb` is unchanged

#### Scenario: Limit survives restart
- **WHEN** the server restarts after the limit was saved
- **THEN** the saved limit is loaded and applied

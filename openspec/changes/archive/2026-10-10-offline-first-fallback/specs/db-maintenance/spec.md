# Delta: db-maintenance

## ADDED Requirements

### Requirement: Image cache size limit

The image cache directory (`<cache_dir>/images`) SHALL be pruned to a configured
maximum size. When the total size of cached image files exceeds `max_cache_gb`
(default 2) after an image download or at a maintenance tick, files with the
oldest modification time SHALL be deleted until the total is at or below the
limit, and the freed amount SHALL be logged. A `max_cache_gb` of 0 SHALL disable
size-based pruning entirely.

#### Scenario: Cache exceeds the configured limit

- **WHEN** the image cache totals more than `max_cache_gb`
- **AND** maintenance runs or an image download completes
- **THEN** the oldest-modified files are deleted until the cache totals at most
  the limit
- **AND** the freed amount is logged

#### Scenario: Recently written files survive

- **WHEN** pruning deletes overflow files
- **THEN** files written most recently (the chapter currently being read) remain

#### Scenario: Pruning disabled

- **WHEN** `max_cache_gb` is 0
- **THEN** no files are ever deleted by the size limit

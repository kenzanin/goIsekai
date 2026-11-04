# Spec Delta

## MODIFIED Requirements

### Requirement: Image cache size limit

The image cache directory (`<cache_dir>/images`) SHALL be pruned to a configured
maximum size. When the total size of cached image files exceeds `max_cache_gb`
(default 2) after an image download or at a maintenance tick, files SHALL be
deleted until the total is at or below the limit, and the freed amount SHALL be
logged. Files SHALL be deleted in order of least recent use, where serving a
file from the cache counts as use. Files that belong to a library manga (its
cover and the pages of its chapters) SHALL be deleted only after every other
file has been deleted. A `max_cache_gb` of 0 SHALL disable size-based pruning
entirely.

#### Scenario: Cache exceeds the configured limit

- **WHEN** the image cache totals more than `max_cache_gb`
- **AND** maintenance runs or an image download completes
- **THEN** the least recently used files are deleted until the cache totals at
  most the limit
- **AND** the freed amount is logged

#### Scenario: Recently written files survive

- **WHEN** pruning deletes overflow files
- **THEN** files written most recently (the chapter currently being read) remain

#### Scenario: Pruning disabled

- **WHEN** `max_cache_gb` is 0
- **THEN** no files are ever deleted by the size limit

#### Scenario: Reading a cached file keeps it

- **WHEN** a cached image is served from the disk cache
- **THEN** it ranks as recently used, so it is deleted after files that were
  written later but never read

#### Scenario: Library files are deleted last

- **WHEN** the cache exceeds the limit and holds both library and non-library files
- **THEN** non-library files are deleted first, whatever their age
- **AND** a library manga's cover and chapter pages remain while any non-library file remains

#### Scenario: Library alone exceeds the limit

- **WHEN** only library files remain and the cache still exceeds the limit
- **THEN** the least recently used library files are deleted until the cache is at most the limit

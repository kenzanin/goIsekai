# Spec Delta

## ADDED Requirements

### Requirement: Changed cache limit is applied without a restart
When the hot-reloaded `max_cache_gb` is lower than the previous value and the image cache currently exceeds the new limit, the host SHALL prune the cache down to the new limit without waiting for a restart or the next maintenance tick. The prune SHALL run off the request that saved the setting and SHALL log the freed amount. Raising the limit, or a reload that does not change it, SHALL NOT delete any file. A limit of 0 SHALL delete nothing.

#### Scenario: Lowering the limit prunes
- **WHEN** the cache totals 3 GB and the user lowers `max_cache_gb` to 2
- **THEN** the cache is pruned to at most 2 GB shortly after the config reload
- **AND** the freed amount is logged

#### Scenario: Raising the limit deletes nothing
- **WHEN** the user raises `max_cache_gb`
- **THEN** no cached file is deleted

#### Scenario: Reload without a change
- **WHEN** the config file is reloaded and `max_cache_gb` is the same as before
- **THEN** no prune is triggered by the reload

#### Scenario: Limit of zero
- **WHEN** the user sets `max_cache_gb` to 0
- **THEN** no file is deleted

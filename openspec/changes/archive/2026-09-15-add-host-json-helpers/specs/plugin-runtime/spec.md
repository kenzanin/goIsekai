## MODIFIED Requirements

### Requirement: Safe Lua stdlib subset
The host SHALL expose only the safe Lua standard subsets (`string`, `table`, `math`, and `os.time`/`os.date`, `os.clock`); file, IO, process, and unrestricted `os` access SHALL NOT be registered. JSON handling SHALL NOT be registered as a Lua global: the host SHALL provide it through the shared `host.json` helper group instead, so Lua and JS plugins use one JSON surface.

#### Scenario: Unsafe library unavailable
- **WHEN** a Lua plugin calls `io.open` or `os.execute`
- **THEN** the call errors with "attempt to index a nil value" (library not registered) rather than touching the host filesystem or processes

#### Scenario: JSON global no longer registered
- **WHEN** a Lua plugin calls `json.encode` or `json.decode`
- **THEN** the call errors with "attempt to index a nil value" (the global is not registered) and the plugin is expected to call `host.json.encode` or `host.json.decode` instead

#### Scenario: JSON remains reachable through the host surface
- **WHEN** a Lua plugin calls `host.json.decode` with a valid document
- **THEN** it receives the decoded value, so removing the global costs plugins no JSON capability

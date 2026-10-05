# Spec Delta

## ADDED Requirements

### Requirement: Template cache is the default render path

On a normal run, the system SHALL serve every render from the bytecode cache built at startup and SHALL NOT re-read or recompile template sources while serving requests. Re-reading templates from disk SHALL happen only when hot reload is explicitly enabled, and SHALL be limited to templates whose content has changed since the last render.

#### Scenario: Steady-state render does not touch disk
- **WHEN** a view route renders repeatedly with hot reload disabled
- **THEN** each render executes the cached bytecode and no template source file is read

#### Scenario: Hot reload disabled is the default
- **WHEN** the server starts without an explicit hot reload setting
- **THEN** the engine runs in cache mode, and an edit to a template file is not visible until the process restarts

#### Scenario: Changed template is picked up under hot reload
- **WHEN** hot reload is enabled and a template file's contents differ from the previously loaded contents
- **THEN** that template is recompiled before its next render, and templates whose contents are unchanged continue to serve from cache

#### Scenario: Unchanged templates are not recompiled under hot reload
- **WHEN** hot reload is enabled and only one template file was edited
- **THEN** only the edited template is recompiled; the remaining templates keep their existing cached bytecode

### Requirement: Template cache is safe under concurrent renders

The system SHALL allow concurrent requests to render templates without corrupting the cache. A cache refresh triggered by hot reload SHALL NOT be observed partially by an in-flight render.

#### Scenario: Concurrent renders during a refresh
- **WHEN** several requests render views at the same time one template file changes under hot reload
- **THEN** every request completes with either the previous or the refreshed template, and none observes a partially written cache entry

#### Scenario: A template that fails to recompile
- **WHEN** hot reload is enabled and an edited template no longer compiles
- **THEN** the previously cached bytecode for that template continues to serve requests and the failure is logged
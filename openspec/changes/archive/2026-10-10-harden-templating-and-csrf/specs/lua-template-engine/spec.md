# Spec Delta

## ADDED Requirements

### Requirement: Template cache is the default render path

On a normal run, the system SHALL serve every render from the bytecode and source caches built at startup, and SHALL NOT read any template file from disk while serving requests. Re-reading templates from disk SHALL happen only when hot reload is explicitly enabled, and SHALL be limited to templates whose content has changed since the last check.

#### Scenario: Steady-state render does not touch disk
- **WHEN** a view route renders repeatedly with hot reload disabled
- **THEN** every render is served from the in-process caches and no template file is read from disk

#### Scenario: Modules pulled in by require are served from the cache too
- **WHEN** a view requires partial modules and renders repeatedly with hot reload disabled
- **THEN** those partials are served from the source cache and their files are not read from disk on each render

#### Scenario: Hot reload disabled is the default
- **WHEN** the server starts without an explicit hot reload setting
- **THEN** the engine runs in cache mode, and an edit to a template file is not visible until the process restarts

#### Scenario: Changed template is picked up under hot reload
- **WHEN** hot reload is enabled and a template file's contents differ from the previously loaded contents
- **THEN** that template is recompiled before its next render, and templates whose contents are unchanged keep their existing cache entries

#### Scenario: Unchanged templates are not recompiled under hot reload
- **WHEN** hot reload is enabled and only one template file was edited
- **THEN** only the edited template is recompiled; the remaining templates keep their existing cache entries

### Requirement: Compiling templates is bounded per render

A module that a template pulls in with require SHALL be compiled from cached source rather than recompiled from disk, so the per-render cost of a page view stays bounded by the view and layout themselves. Eliminating the remaining per-render compilation is not part of this capability.

#### Scenario: Repeated renders do not recompile partials from disk
- **WHEN** a view that requires several partial modules is rendered many times with hot reload disabled
- **THEN** no template file is read from disk by any of those renders

### Requirement: Template cache is safe under concurrent renders

The system SHALL allow concurrent requests to render templates without corrupting the cache. A cache refresh triggered by hot reload SHALL NOT be observed partially by an in-flight render.

#### Scenario: Concurrent renders during a refresh
- **WHEN** several requests render views at the same time one template file changes under hot reload
- **THEN** every request completes with either the previous or the refreshed template, and none observes a partially written cache entry

#### Scenario: A template that fails to recompile
- **WHEN** hot reload is enabled and an edited template no longer compiles
- **THEN** the previously cached bytecode for that template continues to serve requests and the failure is logged

## Known Limitation

A module reached through `require` is still *compiled* on each render, because lunar's `ScriptOpener` interface yields source text (`io.ReadCloser`) rather than a precompiled prototype, so a bytecode-level cache for required modules is not reachable through the library's API. The disk read is eliminated; the compilation is not. Removing it would mean either keeping one Lua state across requests or preloading modules into every new state, and both trade a correctness risk (state leaking between requests) or equal work for the remaining cost. Both are left for a future change, to be taken only if profiling shows the per-render compile is material.
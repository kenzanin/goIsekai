# plugin-engine-registry Specification (delta)

## MODIFIED Requirements

### Requirement: Engine registration by kind

The host SHALL resolve a plugin runtime through a single registry keyed by the plugin's `kind`, rather than by branching on the kind at each call site. Registering a runtime SHALL be a single operation that makes that kind resolvable everywhere — load, invoke, and teardown — without editing those paths.

#### Scenario: Registered kind resolves for every operation

- **WHEN** a runtime is registered under a kind
- **THEN** a plugin of that kind can be loaded, invoked, and unloaded through the same registry lookup, with no per-kind branch added to the load, invoke, or teardown paths

#### Scenario: All shipped kinds are registered

- **WHEN** the host starts
- **THEN** the kinds `lua`, `js`, `native`, `yaegi`, and `wasm` are each resolvable through the registry

#### Scenario: Unregistered kind is rejected

- **WHEN** a plugin declares a kind with no registered runtime
- **THEN** the host returns an error naming the plugin and the unsupported kind, and SHALL NOT panic

## ADDED Requirements

### Requirement: Shared-library kind names the runtime, not the language

The kind identifying a native shared-library plugin SHALL name the runtime the host uses — `native` — rather than the language the library was compiled from. A shared library loaded through the host's native loader SHALL be classified identically whether it was written in Go, C, or C++.

#### Scenario: Shared library classifies as native

- **WHEN** a plugin ships a native shared library as its entry
- **THEN** the host registers it under the kind `native`, regardless of the language that produced the library

#### Scenario: Native kind reaches the UI as native

- **WHEN** the plugin list is rendered
- **THEN** a shared-library plugin shows the native badge, resolved from the kind `native`

#### Scenario: Kind survives a restart unchanged

- **WHEN** the host restarts with a shared-library plugin installed
- **THEN** the plugin's kind is `native` again, because the kind is derived from the plugin's entry at load time and is never persisted under its previous spelling

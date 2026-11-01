# plugin-engine-registry Specification

## Purpose

How the host registers plugin runtimes by kind and resolves one per plugin, so adding, replacing, or removing an engine touches a single registration point instead of every dispatch site.

## Requirements

### Requirement: Engine registration by kind

The host SHALL resolve a plugin runtime through a single registry keyed by the plugin's `kind`, rather than by branching on the kind at each call site. Registering a runtime SHALL be a single operation that makes that kind resolvable everywhere — load, invoke, and teardown — without editing those paths.

#### Scenario: Registered kind resolves for every operation

- **WHEN** a runtime is registered under a kind
- **THEN** a plugin of that kind can be loaded, invoked, and unloaded through the same registry lookup, with no per-kind branch added to the load, invoke, or teardown paths

#### Scenario: All shipped kinds are registered

- **WHEN** the host starts
- **THEN** the kinds `lua`, `js`, `go`, `yaegi`, and `wasm` are each resolvable through the registry

#### Scenario: Unregistered kind is rejected

- **WHEN** a plugin declares a kind with no registered runtime
- **THEN** the host returns an error naming the plugin and the unsupported kind, and SHALL NOT panic

### Requirement: Uniform load and close lifecycle

The host SHALL drive every runtime's teardown through the same registry operation, without the caller knowing the kind. Unloading a plugin SHALL release that runtime's resources and leave the plugin registered, re-loadable on its next call.

#### Scenario: Unload dispatches to the runtime's own close

- **WHEN** a loaded plugin of any kind is unloaded
- **THEN** the host closes it through the registry and the plugin reverts to registered-only state

#### Scenario: Reload closes the previous instance once

- **WHEN** a plugin is reloaded
- **THEN** the previous runtime instance is closed exactly once, with no duplicated per-kind teardown

#### Scenario: Unloaded plugin re-loads on next call

- **WHEN** an unloaded plugin receives an ABI call
- **THEN** the host re-instantiates its runtime through the registry, the same way as the first load

### Requirement: JavaScript runtime builds without cgo

The JavaScript runtime SHALL be provided by a pure-Go engine that compiles with `CGO_ENABLED=0`. The production build path SHALL NOT require a C toolchain, a shared library, or any `import "C"` in the dependency graph of the JS runtime.

#### Scenario: Production build stays cgo-free

- **WHEN** the host is built with `CGO_ENABLED=0`
- **THEN** the build succeeds and the JS runtime is included

#### Scenario: Replaced engine leaves no residue

- **WHEN** the JavaScript engine is replaced
- **THEN** the previous engine library no longer appears in the module's dependency graph

### Requirement: JS engine swap preserves the plugin ABI

Replacing the JavaScript engine SHALL NOT change the plugin-visible contract. The five ABI exports SHALL resolve and be callable, and the host helper groups SHALL expose the same functions with the same results as before the swap.

#### Scenario: ABI exports still resolve

- **WHEN** a JS plugin defining the ABI functions is loaded after the swap
- **THEN** each of `search`, `getMangaDetail`, `getChapterList`, `getPageList`, and `getGenres` resolves and is callable

#### Scenario: Helper surface unchanged

- **WHEN** the cross-runtime helper parity test runs after the swap
- **THEN** the JS runtime covers the same host surface as the Lua and wasm runtimes, and the parity test passes unchanged

#### Scenario: Shipped JS plugin keeps working

- **WHEN** the shipped JS plugin runs search, chapter list, and page list against its source
- **THEN** it returns the same shapes it returned before the swap

### Requirement: JS invocation remains bounded

The JavaScript runtime SHALL enforce an execution budget per invocation, so a plugin that never returns cannot hang the host. Exceeding the budget SHALL end the call with a timeout error.

#### Scenario: Non-terminating plugin times out

- **WHEN** a JS plugin invocation runs past the configured budget
- **THEN** the host terminates that invocation and returns a timeout error

#### Scenario: Later calls still work

- **WHEN** a JS invocation has been terminated for exceeding its budget
- **THEN** subsequent invocations of that plugin still run, rather than the VM being left unusable

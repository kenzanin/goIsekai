## Purpose

Enables plugin authors to write manga source plugins in standard Go, interpreted at runtime by Scriggo — no compilation toolchain, no CGO, no TinyGo required.

## ADDED Requirements

### Requirement: Scriggo plugin discovery

The plugin manager SHALL auto-discover `.go` files (or directories containing `main.go`) in the plugins directory and register them as kind `"scriggo"`.

#### Scenario: Plugin directory contains .go file

- **WHEN** a file `myplugin.go` or directory `myplugin/main.go` exists in the plugins directory
- **THEN** the plugin manager registers a plugin with kind `"scriggo"` and the plugin ID derived from the filename or directory name

#### Scenario: Scriggo plugin appears in plugin list

- **WHEN** the host lists all plugins
- **THEN** Scriggo plugins appear with kind `"scriggo"` alongside WASM, Lua, JS, and Go plugins

### Requirement: Scriggo plugin execution

The plugin manager SHALL execute Scriggo plugins via the Scriggo interpreter, dispatching ABI calls to the plugin's exported functions.

#### Scenario: Successful ABI call

- **WHEN** the host calls `SearchManga(filter)` on a Scriggo plugin
- **THEN** the plugin's `SearchManga` function is invoked with the JSON-serialized filter argument and its return value is deserialized as `[]types.Manga`

#### Scenario: Plugin returns error

- **WHEN** a Scriggo plugin function returns an error
- **THEN** the error is propagated to the caller with the plugin ID and function name in the error message

#### Scenario: Plugin panics

- **WHEN** a Scriggo plugin function panics
- **THEN** the panic is caught, converted to an error, and the plugin manager remains functional

### Requirement: Scriggo sandboxing

Scriggo plugins SHALL be sandboxed: no stdlib imports are available unless explicitly whitelisted, and execution is interruptible via context.

#### Scenario: Plugin attempts to import os

- **WHEN** a Scriggo plugin contains `import "os"`
- **THEN** the build fails with an error indicating the package is not available

#### Scenario: Plugin uses whitelisted package

- **WHEN** a Scriggo plugin imports `fmt` (whitelisted for debugging)
- **THEN** the build succeeds and `fmt.Println` output is captured

#### Scenario: Plugin execution timeout

- **WHEN** a Scriggo plugin function runs longer than the configured timeout
- **THEN** the execution is interrupted and an error is returned

### Requirement: Scriggo plugin networking

Scriggo plugins SHALL perform HTTP requests via the host-provided `http_request` function, identical to other runtimes.

#### Scenario: Plugin makes HTTP request

- **WHEN** a Scriggo plugin calls the exposed `hostnet.Get(url)` function
- **THEN** the request is executed via the host's TLS-fingerprinted HTTP client and the response body is returned to the plugin

### Requirement: Lazy loading

Scriggo plugins SHALL be lazily loaded: the interpreter is instantiated on the first ABI call, not at discovery time.

#### Scenario: Plugin registered but not loaded

- **WHEN** a Scriggo plugin is discovered
- **THEN** no interpreter resources are allocated until the first ABI call is made

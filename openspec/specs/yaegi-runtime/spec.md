# Yaegi Plugin Runtime

## Purpose

Enables plugin authors to write manga source plugins in standard Go, interpreted at runtime by Yaegi (github.com/traefik/yaegi) — no compilation toolchain, no CGO required.

## Requirements

### Requirement: Yaegi plugin discovery

The plugin manager SHALL auto-discover directories containing `main.go` in the plugins directory and register them as kind `"yaegi"`.

#### Scenario: Yaegi plugin discovered

- **GIVEN** a plugin directory containing `main.go` with ABI functions
- **WHEN** the plugin manager scans the plugins directory
- **THEN** the plugin manager registers a plugin with kind `"yaegi"` and the plugin ID derived from the directory name

#### Scenario: Yaegi plugin appears in plugin list

- **WHEN** Yaegi plugins are discovered
- **THEN** they appear with kind `"yaegi"` alongside Lua, JS, and Go plugins

### Requirement: Yaegi plugin execution

The plugin manager SHALL execute Yaegi plugins via the Yaegi interpreter, dispatching ABI calls to the plugin's exported functions through load-time generated wrapper shims.

- ABI functions take one string arg and return `(string, error)`; `Init` takes no arg and returns `string` or `(string, error)`.
- The host generates a per-function wrapper (`gskCall_<Fn>`) that collapses the multi-return into a single string, panicking on error so it surfaces as an Eval error — calling a multi-return function directly via `Eval` would silently drop the error value.
- Calls are bounded by `invokeTimeout` via `EvalWithContext`; an infinite loop aborts with a deadline error.

#### Scenario: Host calls Search on a Yaegi plugin

- **WHEN** the host calls `Search(filter)` on a Yaegi plugin
- **THEN** the plugin's `Search` runs in the interpreter and its JSON string result is returned to the host

#### Scenario: Yaegi plugin function returns an error

- **WHEN** a Yaegi plugin function returns an error
- **THEN** the host receives a plugin error prefixed with the plugin ID and function name

#### Scenario: Yaegi plugin function panics

- **WHEN** a Yaegi plugin function panics
- **THEN** the panic is recovered at the VM boundary and reported as a plugin error, not a host crash

#### Scenario: Yaegi plugin times out

- **WHEN** a Yaegi plugin function runs longer than the configured timeout
- **THEN** the call is aborted and the host reports a timeout for that plugin and function

### Requirement: Yaegi sandboxing

Yaegi plugins SHALL be sandboxed: imports are validated at load time — only the Go standard library and the synthetic `hostnet` package are allowed; any third-party (`github.com/...`, `goisekai/...`) import fails the load. Execution is interruptible via `EvalWithContext`.

#### Scenario: Yaegi plugin imports a third-party package

- **WHEN** a Yaegi plugin source imports `github.com/foo/bar`
- **THEN** plugin loading fails with an "imports disallowed package" error

#### Scenario: Yaegi plugin imports hostnet

- **WHEN** a Yaegi plugin imports the synthetic `hostnet` package
- **THEN** the import is allowed and `hostnet.Get`/`hostnet.Post` are exposed

### Requirement: Yaegi plugin networking

Yaegi plugins SHALL perform HTTP requests via the host-provided `hostnet.Get`/`hostnet.Post` bridge, which routes through the same TLS-fingerprinted per-plugin proxy as other runtimes. The bridge exposes only stdlib-compatible types because Yaegi cannot parse bogdanfinn/fhttp source.
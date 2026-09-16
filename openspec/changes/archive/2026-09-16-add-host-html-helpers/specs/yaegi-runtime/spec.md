## MODIFIED Requirements

### Requirement: Yaegi sandboxing

Yaegi plugins SHALL be sandboxed: imports are validated at load time — only the Go standard library and the synthetic `hostnet` package are allowed; any third-party (`github.com/...`, `goisekai/...`) import fails the load. Execution is interruptible via `EvalWithContext`. Beyond the HTTP bridge, the synthetic `hostnet` package SHALL also expose the HTML document handle the Lua and JS runtimes use, so a Yaegi plugin reads scraped markup with the same lookups and the same results. The handle SHALL be built from markup passed in as a string and SHALL NOT read a file or open a connection, so it widens no capability the sandbox withholds.

#### Scenario: Yaegi plugin imports a third-party package

- **WHEN** a Yaegi plugin source imports `github.com/foo/bar`
- **THEN** plugin loading fails with an "imports disallowed package" error

#### Scenario: Yaegi plugin imports hostnet

- **WHEN** a Yaegi plugin imports the synthetic `hostnet` package
- **THEN** the import is allowed and `hostnet.Get`/`hostnet.Post` are exposed

#### Scenario: Yaegi plugin parses markup through the host bridge

- **WHEN** a Yaegi plugin calls the host bridge's parse function with a markup string and runs a lookup on the returned handle
- **THEN** it receives the same value the Lua and JS runtimes receive for the same markup and selector

#### Scenario: The HTML bridge reads no files

- **WHEN** a Yaegi plugin passes a filesystem path to the host bridge's parse function instead of markup
- **THEN** it receives a handle parsed from that literal text and no host file is read

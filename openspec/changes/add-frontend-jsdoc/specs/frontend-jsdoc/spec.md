# frontend-jsdoc Specification (Delta)

## ADDED Requirements

### Requirement: Hand-written frontend JS carries JSDoc annotations

Every top-level function and significant closure in the two hand-written frontend sources (`cmd/goisekai/frontend/lib/alpine-components.js`, `cmd/goisekai/frontend/lib/reader.js`) SHALL carry a JSDoc comment block declaring `@param` (with type) for each parameter and `@returns` for non-void functions.

#### Scenario: Annotated function passes static review

- **WHEN** a top-level function in either file is inspected
- **THEN** it has a JSDoc block whose `@param` count matches its parameter list, and `@returns` is present unless the function returns nothing

### Requirement: Shared object shapes are typed once via @typedef

State and preference objects used across functions (reader prefs, viewer state) SHALL be declared once as a `@typedef` at the top of the file that owns them, and functions taking them SHALL reference the typedef name instead of repeating inline object types.

#### Scenario: Reader prefs typedef exists

- **WHEN** `reader.js` is inspected
- **THEN** a single `@typedef` describes the server-prefs object shape, and functions consuming it reference that name

### Requirement: Annotations are comments only — no behavior change

Adding JSDoc SHALL NOT alter runtime behavior: no code moves, no renames, no new syntax. `TestFrontendJSParses` MUST stay green and the `.br` siblings regenerated via `just br` MUST match the annotated sources.

#### Scenario: Parse test and brotli siblings stay green

- **WHEN** the annotated sources are committed and the gate runs
- **THEN** `go test ./internal/httpserver/ -run TestFrontendJSParses` passes and `just br` produces siblings matching the sources

### Requirement: Vendored minified files are out of scope

`alpine.min.js` and `mermaid.min.js` SHALL NOT be annotated or modified in any way.

#### Scenario: Minified files untouched

- **WHEN** the change's diff is reviewed
- **THEN** no minified `*.min.js` file appears in it

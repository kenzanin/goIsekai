# Spec Delta — host-regex-helpers

## Purpose

The host exposes a shared regex helper surface to Lua, JS and wasm plugins so they
stop reimplementing string matching. This capability fixes and documents the exact
row/capture shape each helper returns, so a plugin decodes the same answer in every
runtime instead of silently getting an empty result.

## ADDED Requirements

### Requirement: Consistent find_all row shape

The host SHALL return the result of `regex.find_all` as an array of rows, where each
row is an array of the pattern's capture groups. A pattern with exactly one capture
group SHALL produce a one-element array per row, not a bare string. A pattern with no
capture groups SHALL produce a one-element array containing the whole match. The row
shape SHALL be identical across the Lua, JS and wasm runtimes for the same pattern and
subject.

#### Scenario: Single-capture pattern returns an array row
- **WHEN** `regex.find_all` is called with a pattern containing one capture group and a subject with three matches
- **THEN** it returns three rows, each a one-element array holding that match's capture, never a bare string

#### Scenario: No-capture pattern returns the whole match
- **WHEN** `regex.find_all` is called with a pattern containing no capture groups
- **THEN** each row is a one-element array containing the whole match

#### Scenario: Multi-capture pattern unchanged
- **WHEN** `regex.find_all` is called with a pattern containing two or more capture groups
- **THEN** each row is an array of that match's capture groups in order

#### Scenario: Runtime parity
- **WHEN** a Lua plugin, a JS plugin and a wasm plugin each call `regex.find_all` with the same pattern and subject
- **THEN** all three receive the same row shape

### Requirement: find_all failure is loud, not empty

When a `regex.find_all` result cannot be decoded into the expected row shape, the host
SHALL log a structured warning naming the plugin and pattern, rather than returning a
silently empty result. A decode failure SHALL NOT surface to the caller as an empty
list without a host-side warning.

#### Scenario: Decode failure logs a warning
- **WHEN** a plugin's decoded `regex.find_all` result does not match the expected row shape
- **THEN** the host logs a warning naming the plugin and pattern before returning

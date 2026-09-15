# Host JSON Helpers Specification

## Purpose

Give Lua and JS plugins the same host-backed JSON codec, so both runtimes parse and produce JSON identically and plugin authors write one JSON call rather than one per language.

## Requirements

### Requirement: Host JSON decode native
The host SHALL expose `host.json.decode(string)` in both the Lua and JS plugin runtimes, converting a JSON document held in a string into the runtime's native value: a Lua table for Lua, an object or array for JS. The native SHALL reject a non-string argument, and SHALL reject input that is not valid JSON, reporting the failure to the caller rather than returning a partial or empty value.

#### Scenario: JSON object decoded in Lua
- **WHEN** a Lua plugin calls `host.json.decode('{"query":"solo"}')`
- **THEN** it receives a table whose `query` field is `solo`

#### Scenario: JSON array decoded in JS
- **WHEN** a JS plugin calls `host.json.decode('[1,2,3]')`
- **THEN** it receives an array of length 3

#### Scenario: Invalid JSON rejected
- **WHEN** a plugin calls `host.json.decode` with a string that is not valid JSON
- **THEN** the call fails with an error naming the JSON parse failure and no value is returned

#### Scenario: Non-string argument rejected
- **WHEN** a plugin calls `host.json.decode` with a number instead of a string
- **THEN** the call fails and the plugin sees an error about the argument instead of a decoded value

### Requirement: Host JSON encode native
The host SHALL expose `host.json.encode(value)` in both the Lua and JS plugin runtimes, converting the runtime's native value into a JSON string. The native SHALL reject a value that cannot be represented as JSON, reporting the failure to the caller rather than returning a partial document.

#### Scenario: Lua table encoded
- **WHEN** a Lua plugin calls `host.json.encode` with a table carrying `{id = "L1", title = "Lua"}`
- **THEN** it receives a JSON string whose parsed form carries those `id` and `title` fields

#### Scenario: JS object encoded
- **WHEN** a JS plugin calls `host.json.encode` with `{source: "example", items: []}`
- **THEN** it receives the equivalent JSON string

#### Scenario: Unrepresentable value rejected
- **WHEN** a plugin passes a value that cannot be represented as JSON, such as a function
- **THEN** the call fails and the plugin sees the encode error instead of a partial document

### Requirement: Identical JSON semantics across runtimes
Both runtimes SHALL implement `host.json` with the same codec, so identical input yields identical output: the same encoding of numbers, the same treatment of keys, and the same error text for the same failure. A JSON call SHALL be movable between a Lua and a JS plugin without change.

#### Scenario: Same value, same encoded output
- **WHEN** a Lua plugin and a JS plugin each call `host.json.encode` with the equivalent value `{a = 1, b = "x"}`
- **THEN** both receive the same JSON string

#### Scenario: Same document, equivalent decoded values
- **WHEN** a Lua plugin and a JS plugin each call `host.json.decode` with the same document
- **THEN** both observe equivalent values in their own native types

#### Scenario: Same failure, same error text
- **WHEN** both runtimes call `host.json.decode` with the same malformed document
- **THEN** both report the same error text

### Requirement: host.json present in every script runtime
The `host` value exposed to plugins SHALL include a `json` group in both the Lua and JS runtimes, alongside the existing `text`, `codecs`, `crypto` and `http` groups, so the shared helper surface is uniform across languages.

#### Scenario: json group present in both runtimes
- **WHEN** a Lua plugin and a JS plugin each inspect the `host` value
- **THEN** both find a `json` group exposing `decode` and `encode`

#### Scenario: Existing host groups unaffected
- **WHEN** a plugin calls an already-shipped helper such as `host.text.strip_html` or `host.codecs.base64_encode`
- **THEN** it behaves exactly as it did before the JSON group was added

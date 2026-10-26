# Proposal — fix-host-regex-find-all-shapes

## Why

Plugins routinely decode the host's `regex.find_all` answer into the wrong shape and
get an empty result **with no error**, because the host renders a one-capture-group
row as a bare string rather than an array. This has silently zeroed chapter pages
(mangabuddy) and search results across multiple plugins. It is the single most
repeated plugin bug in the OKF bundle, so the helper's output shape should be made
unambiguous and the failure should surface loudly instead of returning empty.

## What Changes

- **BREAKING (shape only)**: make `regex.find_all` return a **consistent
  `[][]string`** — every row is an array of capture groups, even when the pattern
  has exactly one group. Remove the `wasmRegexRow` special case that flattens a
  1-capture row to a bare string (`internal/pluginmanager/wasm_host_helpers.go`).
  When the pattern has no capture groups, each row is the single-element array of
  the whole match (unchanged from `pluginutil.RegexFindAll`).
- Keep `regex.find_all` and the older `abi.find_all_pairs` aligned so Lua, JS and
  wasm plugins all observe the same row shape for the same pattern.
- Emit a structured host-side warning when a plugin's `regex.find_all` result fails
  to decode into the expected shape (defense-in-depth, so a future drift logs
  instead of vanishing).

## Capabilities

### New Capabilities

- `host-regex-helpers`: the regex helper surface (`regex.find`, `regex.find_all`,
  `regex.replace`, `regex.replace_func`) exposed to Lua/JS/wasm plugins, and the
  exact row/capture shape each helper returns.

### Modified Capabilities

(none — regex helpers were previously specified implicitly; this change gives them
a first-class capability and fixes the row-shape requirement.)

## Impact

- Affected code: `internal/pluginmanager/wasm_host_helpers.go` (`wasmRegexRow`),
  `internal/pluginutil/regex.go` (`RegexFindAll`), wasm/Lua/JS dispatch.
- Plugin impact: any plugin that already flattened 1-group rows by hand (correctly)
  keeps working; plugins that decoded a 1-group row as an array now get the array.
- OKF references: `three-lua-plugins-converted-to-tinygo-wasm`,
  `odin-wasm-plugin-port-candidates`, `log.md` find_all_pairs trap.

# Design — fix-host-regex-find-all-shapes

## Problem

`pluginutil.RegexFindAll` already returns `[][]string` (rows of capture groups), but
the wasm/JS dispatch runs each row through `wasmRegexRow`
(`internal/pluginmanager/wasm_host_helpers.go`), which **flattens a single-element
row to a bare string**:

```go
func wasmRegexRow(row []string) any {
    switch len(row) {
    case 0:  return nil
    case 1:  return row[0]   // <-- the trap: 1 capture group → bare string
    default: return row
    }
}
```

So a one-group pattern like `(?s)<dd>(.*?)</dd>` decodes to a string per row, not a
1-element array. A plugin written to expect `[][]string` (the shape every multi-group
pattern produces) decodes **nothing and no error** → empty result. This silently zeroed
mangabuddy's chapter pages and is the most-repeated bug in the OKF bundle.

## Change

Remove the `wasmRegexRow` special case so `regex.find_all` always returns `[][]string`:

- 0 capture groups → each row is `[wholeMatch]` (1-element array) — already what
  `RegexFindAll` does via `whole := re.NumSubexp() == 0`.
- 1 capture group → each row is `[capture1]` (1-element array) — **no longer flattened**.
- N capture groups → `[capture1, …, captureN]` — unchanged.

Keep `find_all_pairs` (the older abi name) observing the same shape so Lua/JS/wasm
agree. The change is shape-only and non-breaking for plugins that already flatten
1-group rows by hand (they get an array with one element and index `[0]`).

## Loud-failure defense

`RegexFindAll` is the single choke point. Add a host-side structured warning when a
row cannot be interpreted (len 0, or a non-`[]string`/`[]any` row) so a future shape
drift logs under `plugin=` and `pattern=` instead of returning empty. This is
belt-and-braces behind the fixed shape.

## Verification

- Unit test in `internal/pluginutil/regex.go` covering 0/1/2/N-capture patterns and
  assert every row is `[]string` (specifically that a 1-group row is a 1-element
  slice, not a string).
- Regression: the wasm dispatch test asserting `regex.find_all` output shape parity
  across runtimes.
- Re-run the live mangabuddy chapter page fetch to confirm pages are non-empty.

## Risks

- A plugin that *depended* on the bare-string flattening (expects a string for a
  1-group row) would now get a 1-element array. Mitigated by checking all in-repo
  examples and noting the shape change in the OKF concept. Grep shows the known-good
  parsers (lhtranslation `firstCapAll`, `firstCap`) already handle both shapes via a
  fallback, so in-repo plugins are safe.

## Non-goals

- Not changing RE2 semantics or adding new regex helpers — only the row shape and the
  loud-failure logging.

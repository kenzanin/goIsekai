# Tasks — fix-host-regex-find-all-shapes

## 1. Make find_all rows consistent

- [x] 1.1 Remove the single-element flattening in `wasmRegexRow`
      (`internal/pluginmanager/wasm_host_helpers.go`) so every row stays a
      `[]string` (0-capture → `[wholeMatch]`, 1-capture → `[capture1]`, N → captures).
- [x] 1.2 Confirm `pluginutil.RegexFindAll` (`internal/pluginutil/regex.go`) already
      emits consistent `[][]string` rows and align `find_all_pairs` to the same shape.
- [x] 1.3 Add a structured host warning when a `regex.find_all` row cannot be decoded
      into `[]string` (log `plugin=` + `pattern=`), instead of returning empty silently.

## 2. Tests

- [x] 2.1 Unit test in `internal/pluginutil/regex.go`: 0/1/2/N-capture patterns each
      return `[]string` rows (assert 1-group row is a 1-element slice, not a string).
- [ ] 2.2 Regression test asserting `regex.find_all` shape parity across wasm/Lua/JS
      dispatch.
- [x] 2.3 Live-verify mangabuddy chapter pages are non-empty after the change.

## 3. Finish

- [x] 3.1 `go test ./internal/pluginutil/...` and `golangci-lint run` pass.
- [x] 3.2 Update the OKF concept (`plugin-failure-patterns` Pattern 1 documents the
      fixed shape) documenting the fixed shape.
- [x] 3.3 `openspec validate fix-host-regex-find-all-shapes`; commit + push.

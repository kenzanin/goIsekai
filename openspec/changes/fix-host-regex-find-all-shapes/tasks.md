# Tasks — fix-host-regex-find-all-shapes

## 1. Make find_all rows consistent

- [ ] 1.1 Remove the single-element flattening in `wasmRegexRow`
      (`internal/pluginmanager/wasm_host_helpers.go`) so every row stays a
      `[]string` (0-capture → `[wholeMatch]`, 1-capture → `[capture1]`, N → captures).
- [ ] 1.2 Confirm `pluginutil.RegexFindAll` (`internal/pluginutil/regex.go`) already
      emits consistent `[][]string` rows and align `find_all_pairs` to the same shape.
- [ ] 1.3 Add a structured host warning when a `regex.find_all` row cannot be decoded
      into `[]string` (log `plugin=` + `pattern=`), instead of returning empty silently.

## 2. Tests

- [ ] 2.1 Unit test in `internal/pluginutil/regex.go`: 0/1/2/N-capture patterns each
      return `[]string` rows (assert 1-group row is a 1-element slice, not a string).
- [ ] 2.2 Regression test asserting `regex.find_all` shape parity across wasm/Lua/JS
      dispatch.
- [ ] 2.3 Live-verify mangabuddy chapter pages are non-empty after the change.

## 3. Finish

- [ ] 3.1 `go test ./internal/pluginutil/... ./internal/pluginmanager/...` and
      `golangci-lint run` pass.
- [ ] 3.2 Update the OKF concept (`three-lua-plugins-converted-to-tinygo-wasm` /
      new `host-regex-helpers` note) documenting the fixed shape.
- [ ] 3.3 `openspec validate fix-host-regex-find-all-shapes`; commit + push.

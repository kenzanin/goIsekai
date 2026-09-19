## 1. Bridge: priority-aware admission

- [ ] 1.1 Add a `Prio` type (constants `PrioLow`, `PrioHigh`) and replace
      `hostSem` with a per-host `hostLanes` struct: two channels
      (`high` cap 2, `low` cap 1) plus FIFO wait lists; admission drains
      high waiters before low. Verify: `go build ./internal/...`.
- [ ] 1.2 Change `GetImage` to take a `Prio` parameter and acquire the
      matching lane; `paceImage` and the retry loop stay as-is inside the
      lane. Verify: existing `internal/bridge` tests pass
      (`CGO_ENABLED=0 go test ./internal/bridge/`).
- [ ] 1.3 Add a unit test proving ordering: with a host saturated by low
      fetches, a high fetch is admitted before queued low fetches; and
      same-priority callers keep arrival order. Verify: new test in
      `internal/bridge/image_test.go` passes.

## 2. HTTP layer

- [ ] 2.1 Parse `?prio=` in `internal/httpserver/image.go`: `high` →
      high, anything else → low; forward to `GetImage`. Verify:
      `go vet`/`just check` clean.
- [ ] 2.2 Extend the httptest-level test (or add one) covering absent,
      `high`, and garbage `prio` values resolve to the expected lane
      without erroring. Verify: `CGO_ENABLED=0 go test ./internal/httpserver/`.

## 3. Frontend priority wiring

- [ ] 3.1 `cmd/goisekai/frontend/lib/reader.js`: add a `prio` argument to
      `imageUrl()`; `drawPage` sends `high`; `prefetch`, `prefetchNext`,
      `prefetchPrev`, and strip seam appends send `low`. Verify:
      `node --check`, `biome check cmd/goisekai/frontend`, and network tab
      shows `prio=high` only on the drawn page.
- [ ] 3.2 Templates: detail hero cover sends `prio=high`
      (`internal/templates/views/detail.lua`); library/search/updates grid
      thumbs stay parameterless. Verify: `luacheck internal/templates/`
      clean; curl the rendered detail page and grep the hero img tag.

## 4. Verification

- [ ] 4.1 Run `just test` and `just check`; both clean.
- [ ] 4.2 Live check: start the server, open a search with many covers,
      immediately click into a title — the detail cover renders before the
      queued grid covers finish (observe via devtools request ordering or
      debug logs).

# Tasks

## 1. reader.js annotations

- [x] 1.1 Add a `@typedef` for the reader prefs/state object at the top of `reader.js`, then JSDoc (`@param`/`@returns`) every top-level function; verify with `node --check cmd/goisekai/frontend/lib/reader.js`
- [x] 1.2 Regenerate brotli siblings with `just br` and verify `go test ./internal/httpserver/ -run TestFrontendJSParses -count=1` passes

## 2. alpine-components.js annotations

- [x] 2.1 JSDoc every top-level function and significant closure in `alpine-components.js` (reference the prefs typedef from its owning file where shared); verify with `node --check cmd/goisekai/frontend/lib/alpine-components.js`
- [x] 2.2 Regenerate brotli siblings with `just br` and verify `go test ./internal/httpserver/ -run TestFrontendJSParses -count=1` passes

## 3. Gate

- [x] 3.1 `just check && just build` green; restart the app per the deploy recipe and smoke-test the reader + a detail page (HTTP 200, reader still paginates) to confirm zero behavior change
- [x] 3.2 Confirm the diff touches only the two JS files + their `.br` siblings — no `*.min.js` modified

## Workflow follow-up

- Archive the change after the project's review requirements are satisfied.

# Tasks

## 1. Template cache mode

- [x] 1.1 Add a `hot_reload` key to `[app]` in `internal/config`, defaulting to false, and verify a config round-trip test asserts the default is false
- [x] 1.2 Replace the hardcoded `true` at `cmd/goisekai/main.go:165` with the config value, and verify `go build ./cmd/...` succeeds
- [x] 1.3 In `internal/templates/lua_render.go`, make `render` skip the disk walk when hot reload is off and read the prototype under the existing read lock; verify an engine test with a counting `fs.FS` reports zero reads across repeated renders
- [x] 1.4 Add a `hashes map[string][32]byte` beside `protos` in `internal/templates/lua_engine.go`, populated at startup; verify a test asserts a populated hash for every compiled template
- [x] 1.5 Change the hot-reload path to recompile only entries whose SHA-256 content hash changed, publishing `protos` and `hashes` together under the write lock; verify a test edits one template and asserts the other prototype pointers are unchanged
- [x] 1.6 Make the refresh path log and continue when one template fails to compile, keeping its previous entry; verify a test introduces a syntax error and asserts the old bytecode still renders and the failure was logged

## 2. CSRF token issuance and delivery

- [x] 2.1 Add a `csrf.go` in `internal/httpserver` that mints a 32-byte secret at startup and exposes the derived `hex(HMAC-SHA256(secret, "goisekai-csrf-v1"))` constant; verify a unit test asserts the token is non-empty and stable across calls
- [x] 2.2 Inject the token into the data map in `renderPage` for both full and partial renders; verify a view test asserts the rendered full page contains the meta element
- [x] 2.3 Emit `<meta name="csrf-token" content="...">` from `internal/templates/layouts/base.lua`; verify a template render test asserts the meta tag carries the token value passed in data
- [x] 2.4 Add a hidden `csrf_token` input to every `method="post"` form under `internal/templates/`; verify a grep for `<form` shows each posting form has a matching hidden input and no form is left behind

## 3. CSRF enforcement

- [x] 3.1 Add `requireCSRFToken` in `internal/httpserver/middleware.go` that refuses unsafe methods (`POST`, `PUT`, `PATCH`, `DELETE`) on token mismatch, reading the candidate from `X-CSRF-Token`, then the `csrf_token` field, then the `csrf_token` query field, and comparing in constant time; verify unit tests cover missing, wrong, and correct tokens
- [x] 3.2 Return a client-error status with a body naming the endpoint and stating the token was rejected; verify a test asserts the status and that the body is not a bare status line
- [x] 3.3 Log the refusal with the path and reason through `s.logger`; verify a test asserts the log entry contains the refused path
- [x] 3.4 Move the 31 `POST` routes from `internal/httpserver/actions.go` into a `s.Router.Route("/action", ...)` sub-router carrying `requireCSRFToken`; verify a route-table test asserts every action route sits under the guarded group and no view, static, image, or API route does
- [x] 3.5 Verify no action route rejects a correctly-tokenized request, by exercising `mark-read` and `save-settings` through the router in a test and asserting both mutate as before

## 4. Script-driven requests

- [x] 4.1 Send `X-CSRF-Token` from the page meta tag on every `POST` fetch in `cmd/goisekai/frontend/lib/alpine-components.js`; verify `just lint-web` passes and a grep shows no state-changing fetch without the header
- [x] 4.2 Add the header to the `reader.js` progress and chapter-action calls that post without a form; verify each call site now sets the header and `just lint-web` still passes
- [x] 4.3 Verify the full round trip live: load `/view/library` in a browser, toggle library membership, and confirm the entry persists after a reload (this exercises template render, meta emission, form field, and middleware together)
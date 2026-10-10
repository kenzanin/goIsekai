# Design

## Context

The template engine compiles every `.lua` file at startup into `LuaEngine.protos` (`internal/templates/lua_engine.go:40`), then `render` in `internal/templates/lua_render.go` re-runs that whole walk when `devMode` is set. `cmd/goisekai/main.go:165` passes `true` unconditionally, so the startup cache is built and then immediately thrown away on every request.

There is no session or login system. The reader is self-hosted, `/api` is gated by `requireAPIKey` (`internal/httpserver/api.go:44`) only when `-apiKey` is set, and the 31 `POST /action/*` routes in `internal/httpserver/actions.go` are ungated in all configurations. Callers reach those routes two ways: 24 plain `method="post"` HTML forms, and script-driven `fetch` in `cmd/goisekai/frontend/lib/alpine-components.js` that serialises `new FormData(form)`.

A hard constraint for this repo: production Go files under `internal/`, `pkg/`, `cmd/` stay under 200 lines, and the build is `CGO_ENABLED=0` with no new dependencies.

## Goals / Non-Goals

**Goals:**
- Serve renders from the startup cache; make disk re-reads an opt-in development mode.
- Refuse cross-site mutations on `/action/*` without breaking either calling convention (plain forms and `fetch`).
- Stay inside the existing stack and the 200-line file budget.

**Non-Goals:**
- Any change to `/api` auth. The API key is a separate mechanism and stays as-is.
- Origin/`Referer` allow-listing as the primary defence — it is unreliable for `file://` origins and trivially bypassed by header suppression, so it is not used.
- Session cookies or per-user tokens. There are no users.
- Replacing the Lua engine or its helper set.

## Decisions

### Hot reload becomes an explicit opt-in, defaulting off

Replace the hardcoded `true` at `main.go:165` with a config-derived flag (`[app] hot_reload = false`, reusing the existing hot-reload polling the config already does for `goisekai.ini`). Under the default, `render` executes `e.protos[name]` directly and never touches `templatesFS`.

Rejected: keeping hot reload always on and adding a file-hash check on every render. It still walks the tree on every request, so the cost only shrinks, and the walk is the thing being paid per page view.

Rejected: an explicit compile-at-first-use cache. It trades a per-render walk for a first-render-per-template walk and adds a second cache to reason about; the startup walk is already paid once.

Rejected: a Memcached-style external cache (e.g. `bradfitz/gomemcache`). A compiled `*lua.Prototype` holds bytecode pointers into the runtime and cannot be serialised, so it could not be stored; and a TCP round trip costs more than reading a small `.lua` file that the OS page cache already holds. It would also add a daemon dependency to a single-binary local-first app. The existing on-disk image cache and the SQLite `plugin_cache` table already cover the two cases in this project that genuinely need serialisation.

### The require path needs its own source cache

`protos` alone is not the whole render. Views pull in partials with `require("partials.detail_chapters")`, and `newVM` installs `lua.FSLoader(e.templatesFS)`, so every render re-read and re-compiled each required partial — measured at four extra file reads per render of `views/library` before this change.

lunar's `ScriptOpener` returns an `io.ReadCloser`, i.e. source text, not a `*lua.Prototype`. There is therefore no loader hook that can hand `require` a precompiled prototype. The engine keeps a `sources` map alongside `protos` and builds its VMs with `lua.FuncLoader` serving from it, which removes the disk read from the require path.

The remaining per-render compilation of required partials is accepted for now. Removing it means either reusing one Lua state across requests (the `data` global and helpers would then be shared, so state could leak between requests) or preloading every module into every new state (which costs the same compile it tries to avoid). Neither is a good trade without profiling data, so the limitation is recorded in the spec delta rather than silently solved.

### Hot reload keys off a content hash, not a modification time

The engine keeps `hashes map[string][32]byte` alongside `protos` and `sources`. Under hot reload a refresh walks the tree once, hashes each `.lua`, and recompiles only the entries whose hash moved. Timestamps are not used: mtime granularity is unreliable across the deploy paths this app actually takes (bind-mounted filesystems and file copies preserve or reset mtime unpredictably), whereas content hashing cannot produce a false "unchanged".

Concurrency: `protos`, `sources`, and `hashes` are swapped together under the existing `e.mu` write lock, and `render` reads the prototype under the read lock. A refresh therefore publishes all three maps atomically and an in-flight render sees either the old set or the new one.

A template that fails to compile keeps its previous entry. `loadBytecodes` currently aborts the whole walk on the first compile error; under hot reload a single bad edit must not take every page down, so the refresh path logs and continues.

### CSRF: a per-boot secret, an HMAC token, and double submission

At startup the server mints 32 random bytes into a `csrfSecret`. The token presented to the client is `hex(HMAC-SHA256(secret, "goisekai-csrf-v1"))` — a constant derived once, not per request, which is what keeps it stable across page loads within a run.

Validation compares, in constant time, the candidate against that constant. The candidate is read from, in order: the `X-CSRF-Token` header, then the `csrf_token` form field, then the `csrf_token` query field.

Query-field acceptance exists because a minority of the existing forms navigate rather than post in place; forcing every one of them into a `fetch` rewrite would be a much larger diff than the risk it removes, and a token in a query string does not weaken the defence here because the value is not a secret from the origin — the browser attaches it only for same-origin requests that read it from the page.

Rejected: a per-session cookie with a matching hidden field (the textbook double-submit). This app has no session, so the cookie would have to be minted per visitor with no way to bind it to anything, and the signed-token approach gives the same forgery resistance with no stored state.

Rejected: `Sec-Fetch-Site` / `Origin` header checks. Cheaper, but they are absent on some clients and ignored entirely by older browsers, so they are a defence-in-depth addition rather than the gate. Not added here to keep the middleware to one mechanism.

### Middleware mounts on the action group only

The 31 action routes move into a `s.Router.Route("/action", ...)` sub-router carrying `requireCSRFToken`, mirroring how `/api` carries `requireAPIKey`. View, static, image, and API routes are untouched, so no GET path can be broken by this change.

The middleware checks the method against a small unsafe set (`POST`, `PUT`, `PATCH`, `DELETE`) rather than treating GET as safe, so a future mutating GET is caught by default rather than by remembering to extend a list.

### Token reaches both caller conventions with one change each

`layouts/base.lua` emits `<meta name="csrf-token" content="...">` from a value `renderPage` injects, and every `method="post"` form gains a hidden `csrf_token` input. Hidden inputs flow through `new FormData(form)` automatically, so `alpine-components.js`'s existing fetch path needs only the header set for the handful of fetches that are built without a form.

`renderPage` injects the token into the data map for both full and partial renders, because the SPA swaps partial HTML into a live `<main>` that already has the meta tag; partial output does not need to repeat it.

## Risks / Trade-offs

- **Forgotten form.** A form added without the hidden field starts failing with a client error. Mitigation: the rejection body names the endpoint and says the token was missing, and `internal/httpserver` tests enumerate the action routes so a newly added route cannot mount outside the guarded group unnoticed.
- **Query-string token leaks into logs.** `loggingMiddleware` records the path, and the query string is part of `r.URL.Path` only for the path itself, but a token in a query does reach access logs and `known_hosts`-style tooling. Mitigation: accept the query field only for `POST`, and treat the header and form field as the primary channels.
- **Hot reload off means template edits need a restart.** That is the intended trade and it is what production wants; the flag restores the old loop for development.
- **Both mechanisms are stateless**, so a token minted before a restart is rejected after one. Pages open across a restart will fail their next mutation and need a reload. Acceptable for a locally hosted app; a reload is the correct remedy and the error body says so.

## Migration Plan

1. Ship cache-mode rendering first, behind the flag defaulting off. No behavioural change is observable.
2. Ship the CSRF middleware together with the template and frontend updates in the same commit. Landing the middleware alone breaks every form; landing the forms alone leaves the routes open.
3. Rollback is a single revert of the middleware mount plus the flag default.

## Open Questions

None. The token channel set, the unsafe-method set, and the refresh failure policy are all settled above and would change the spec text if revisited.
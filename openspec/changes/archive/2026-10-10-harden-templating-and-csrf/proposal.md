# Proposal

## Why

Two gaps in the server-rendering layer, both shipping to production today:

1. **Templates recompile on every single render.** `cmd/goisekai/main.go:165` hardcodes `devMode=true` when constructing the engine, and `internal/templates/lua_render.go` recompiles from disk on every `Render`/`RenderPartial` call when that flag is set. The hot-reload behaviour is a development affordance that was never switched off, so every production page view walks the template tree, reads every `.lua` file, and recompiles it.

2. **`/action/*` has no CSRF defence.** 31 `POST` routes mutate library state (delete alt-titles, toggle plugins, clear the cache, save settings, re-exec the binary via `/action/restart`) and none of them validate request origin. `/api` is already gated by `requireAPIKey`, but the HTML form surface is not gated at all, so any page the browser can be made to load can drive those routes.

## What Changes

- **Template compilation becomes cache-first.** Compiled prototypes stay cached across renders. A `dev` flag (or config key) still allows hot reload, but the default for a normal run is cache-first with revalidation only when a template's content hash changes.
- **CSRF protection for state-changing HTML routes.** A per-boot random secret is minted at startup; a token derived from it is exposed to the page (meta tag) and accepted from either a request field or a request header. `POST` requests under `/action/` are rejected when the token is missing or invalid. `GET` routes and `/api` are unaffected.

Nothing here is a breaking change for the UI: forms and fetch calls are updated in the same change so every existing call keeps working.

## Capabilities

### New Capabilities
- `csrf-protection`: token issuance, propagation into the rendered page, and enforcement on state-changing HTML routes.

### Modified Capabilities
- `lua-template-engine`: templates are compiled once and served from cache; re-reading from disk becomes an opt-in development mode rather than the default.

## Impact

- `cmd/goisekai/main.go` — stop hardcoding `devMode=true`; thread the flag from config.
- `internal/templates/lua_engine.go`, `lua_render.go` — cache-first render path with content-hash revalidation.
- `internal/httpserver/middleware.go`, `routes.go` — CSRF middleware, mounted only on the `/action/` group.
- `internal/templates/layouts/base.lua` — emit the token into the page.
- 24 `method="post"` forms across `internal/templates/views/` and `partials/` — carry the token field.
- `cmd/goisekai/frontend/lib/alpine-components.js` — carry the token on fetch-driven `POST`s.
- No new dependencies; `crypto/hmac` and `crypto/rand` from the standard library are sufficient.
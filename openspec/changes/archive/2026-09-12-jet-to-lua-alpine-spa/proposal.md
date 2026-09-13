## Why

CloudyKit/jet is a niche Go template engine with poor community adoption — plugin contributors who already write Lua cannot contribute to UI without learning jet's unfamiliar DSL. Replacing jet with Lua templates unifies the skillset: anyone who writes a plugin can also modify the frontend. Simultaneously, the frontend uses hand-written vanilla JS for toasts, confirms, and live-updating views, with dead HTMX imports and no SPA navigation — every page load is a full reload. Adding Alpine.js replaces all custom JS, removes dead HTMX, and makes navigation instant via fetch + DOM morph.

## What Changes

- **BREAKING**: Replace CloudyKit/jet v6 template engine with a Lua-based template engine (lunar runtime)
- **BREAKING**: Replace all 13 `.jet` template files with `.lua` template files (layouts, views, partials)
- **BREAKING**: Remove CloudyKit/jet dependency from go.mod
- Remove HTMX dependency (imported but unused — zero `hx-*` attributes in templates)
- Remove `app.js` (219 lines): toast system, confirm modal, data-confirm delegation, dead HTMX error handlers — all replaced by Alpine.js components
- Remove inline `<script>` from logs.jet (~109 lines): WebSocket/polling log viewer — replaced by Alpine.js `x-data` component
- Add Alpine.js CDN to base layout with `$store.toast`, `$store.confirm`, and SPA router
- Add per-page Alpine.js components for interactive elements (search debounce, sidebar toggle, profile dropdown, alt-title management, confirm dialogs)
- Add client-side SPA navigation via fetch + Alpine morph (no full page reloads on internal nav)
- Keep `reader.js` (671 lines) untouched — canvas manga reader is a standalone SPA

## Capabilities

### New Capabilities

- `lua-template-engine`: Lua-based server-side template engine replacing CloudyKit/jet — VM lifecycle, data marshaling, Go helper function registration, dev-mode hot reload
- `alpine-spa-frontend`: Alpine.js client-side framework replacing custom JS and HTMX — global stores (toast, confirm), SPA router, per-page interactive components, fetch-based partial rendering

### Modified Capabilities

- `http-server`: Template rendering path changes from jet.Execute to Lua VM execution; new static asset serving for Alpine.js components
- `plugin-abi`: No requirement changes (template engine is host-internal, not plugin-visible)

## Impact

- **Files created**: `internal/templates/lua.go`, `internal/templates/lua_helpers.go`, `internal/templates/lua/*.lua` (13 template files), `cmd/goisekai/frontend/lib/alpine-components.js`
- **Files modified**: `internal/templates/engine.go` (replaced), `internal/httpserver/routes.go` (renderPage change), `cmd/goisekai/main.go` (LuaEngine construction), `internal/httpserver/api_test.go` + `api_alttitles_test.go` (template constructor update)
- **Files removed**: `internal/templates/*.jet` (13 files), `cmd/goisekai/frontend/lib/app.js` (replaced by Alpine stores), HTMX CDN import
- **Dependencies added**: None — lunar is already a dependency; Alpine.js is CDN-loaded
- **Dependencies removed**: `github.com/CloudyKit/jet/v6`
- **DB**: No schema changes
- **API**: No contract changes — all API endpoints serve the same JSON; only HTML rendering changes

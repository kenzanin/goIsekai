## 1. Lua Template Engine Core

- [ ] 1.1 Create `internal/templates/lua.go` with `LuaEngine` struct, `NewLua(devMode bool)`, `Render(w, name, data)` methods, bytecode caching, and dev-mode disk fallback — verify: `go build ./internal/templates/...` compiles
- [ ] 1.2 Create `internal/templates/lua_helpers.go` with `h()` HTML-escape function, `formatDate`, `formatChapterNum`, `getInitials`, `formatBytes`, `pageWindow`, `pageURL` registered as Lua globals — verify: unit test calls each helper and asserts expected output
- [ ] 1.3 Create `internal/templates/lua_marshal.go` with recursive `goToLua(L *lua.LState, v any) lua.LValue` converter (maps, slices, strings, numbers, booleans, nil) — verify: unit test marshals nested map[string]any and asserts Lua table structure
- [ ] 1.4 Write `internal/templates/lua_test.go` covering: successful render, template-not-found error, Lua runtime error, data marshaling round-trip, `h()` escaping — verify: `go test ./internal/templates/...` passes

## 2. Server Integration

- [ ] 2.1 Modify `internal/httpserver/routes.go` `renderPage` to accept `LuaEngine` and `X-Partial` header — full page renders layout+view, partial renders `<main>` only — verify: existing view routes still return valid HTML
- [ ] 2.2 Modify `cmd/goisekai/main.go` to construct `LuaEngine` (dev mode from `--dev` flag) and pass to router — verify: server starts and serves `/view/library` with Lua-rendered HTML
- [ ] 2.3 Add `POST /action/*` endpoints return JSON `{"status":"ok"}` envelope instead of HTML fragments — verify: `curl -X POST /action/toggle-library/...` returns JSON

## 3. Layout and Partial Templates (Lua)

- [ ] 3.1 Create `cmd/goisekai/frontend/templates/layouts/base.lua` — full HTML shell with Alpine.js CDN, Tailwind CSS, `$store.toast`/`$store.confirm` init, SPA router, nav partial — verify: `GET /` returns complete page with Alpine.js script tag
- [ ] 3.2 Create `cmd/goisekai/frontend/templates/layouts/blank.lua` — minimal layout for reader page — verify: `GET /view/manga/{plugin}/{id}/chapter/{n}` renders with blank layout
- [ ] 3.3 Create `cmd/goisekai/frontend/templates/partials/nav.lua` — navigation bar with SPA links, active-state highlighting — verify: nav renders with correct active class on library/search/plugins/settings
- [ ] 3.4 Create `cmd/goisekai/frontend/templates/partials/pagination.lua` — reusable pagination with page numbers and prev/next — verify: search and library pages show correct pagination
- [ ] 3.5 Create `cmd/goisekai/frontend/templates/partials/toast.lua` — Alpine.js toast markup with `x-data`/`x-for`/`x-transition` — verify: toast markup present on every page, no inline `<script>`

## 4. Alpine.js Client-Side Framework

- [ ] 4.1 Create `cmd/goisekai/frontend/lib/alpine-components.js` with `Alpine.store('toast')` (show/dismiss/stack/auto-hide) and `Alpine.store('confirm')` (show Promise/cancel/escape) — verify: JS loads without errors, `Alpine.store('toast').show('test')` displays toast
- [ ] 4.2 Implement SPA `router()` component in base.lua — intercepts `<a>` clicks, fetches with `X-Partial: true`, extracts `<main>`, morphs DOM, updates `pushState`, handles `popstate` — verify: clicking nav links transitions without full reload
- [ ] 4.3 Add `data-confirm` attribute handler in alpine-components.js — delegated click handler shows `$store.confirm` modal before submit/click — verify: "Clear All Cache" button shows confirm modal
- [ ] 4.4 Add global `htmx:responseError` → `$store.toast.show(msg, 'error')` equivalent as `window.addEventListener('unhandledrejection', ...)` — verify: failed fetch shows error toast

## 5. View Templates (Lua) — Small

- [ ] 5.1 Create `views/history.lua` — history list with timestamps, plugin badges, delete action — verify: `GET /view/history` returns correct HTML, timestamps display with `Z` suffix
- [ ] 5.2 Create `views/search.lua` — search input with debounced fetch via Alpine.js `x-model` + `$watch`, plugin dropdown filter, result cards, pagination — verify: search "one piece" returns results, plugin filter works
- [ ] 5.3 Create `views/settings.lua` — config display, clear-cache button with `$store.confirm` — verify: `GET /view/settings` renders, clear-cache shows confirm modal

## 6. View Templates (Lua) — Complex

- [ ] 6.1 Create `views/reader.lua` using blank layout — canvas container, progress tracking — verify: reader loads and `reader.js` initializes on canvas
- [ ] 6.2 Create `views/logs.lua` with Alpine.js `x-data` — WebSocket connection, poll fallback, filter/limit controls, auto-scroll, copy button, colored log levels — verify: logs page streams live entries, filter works, no inline `<script>`
- [ ] 6.3 Create `views/library.lua` — manga cards grid, stats sidebar with collapse/expand `x-show`, plugin count card, duplicate groups accordion, pagination, plugin icon badges — verify: library renders all manga, sidebar toggles, duplicates card shows count
- [ ] 6.4 Create `views/plugins.lua` — plugin cards, file upload, toggle/test actions, TLS Profile dropdown with `x-model`, Test button with toast feedback, Reset button with confirm — verify: plugins page loads, test-profile returns toast, profile dropdown updates
- [ ] 6.5 Create `views/detail.lua` — manga detail (title, status, author, genres, description), alt-title dropdown with add/remove/promote via fetch, chapter list, progress tracking, cache button with confirm — verify: detail page shows all fields, alt-title promote works, chapter links correct

## 7. Cleanup

- [ ] 7.1 Remove all 13 `.jet` template files from `internal/templates/` — verify: `go build ./...` compiles, no remaining `*.jet` references
- [ ] 7.2 Remove `CloudyKit/jet/v6` from `go.mod`/`go.sum` — verify: `go mod tidy` clean, no jet import in any `.go` file
- [ ] 7.3 Remove `cmd/goisekai/frontend/lib/app.js` (replaced by Alpine stores) and HTMX CDN import from base layout — verify: no references to `app.js` or `htmx` in templates or Go code
- [ ] 7.4 Remove `jet.Engine` type and old `New()` constructor from `internal/templates/engine.go` — verify: `go build ./...` compiles
- [ ] 7.5 Update `internal/httpserver/api_test.go` and `internal/httpserver/api_alttitles_test.go` template constructor from `templates.New(devMode)` to `templates.NewLua(devMode)` — verify: `go test ./internal/httpserver/...` passes
- [ ] 7.6 Run `make check` (go fmt, go test -race, modernize, golangci-lint) — verify: exit 0 with no new errors
- [ ] 7.7 Regenerate Tailwind CSS for any new classes (x-cloak, transition utilities) and run `make br` — verify: toast/modal/sidebar transitions render correctly in browser
- [ ] 7.8 Full end-to-end manual verification via CDP: library, search, detail, reader, plugins, settings, logs, history — verify: all pages render, SPA navigation works, toasts appear, confirm modals work

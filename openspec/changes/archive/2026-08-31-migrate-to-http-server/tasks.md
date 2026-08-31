## 1. Dependencies & Project Setup

- [x] 1.1 Add `github.com/go-chi/chi/v5` to go.mod
- [x] 1.2 Add `github.com/CloudyKit/jet/v6` to go.mod
- [x] 1.3 Add `golang.org/x/net/websocket` to go.mod
- [x] 1.4 Download HTMX 2.x JS to `cmd/goisekai/frontend/lib/htmx.min.js`
- [x] 1.5 Create `internal/httpserver/` package with Chi router setup
- [x] 1.6 Create `internal/templates/` directory structure (layouts/, views/, partials/)
- [x] 1.7 Add `host` and `port` config keys to `internal/config/config.go` + CLI flags to `cmd/goisekai/main.go`

## 2. Jet Template Engine Setup

- [x] 2.1 Create `internal/templates/engine.go` — Jet template loader from `go:embed` FS, custom functions (formatDate, formatChapterNum, escapeHtml, getInitials)
- [x] 2.2 Create base layout `templates/layouts/base.jet` — HTML shell, head (Tailwind + HTMX), nav bar, `#content` container
- [x] 2.3 Create `templates/partials/nav.jet` — navigation links (Library, Search, Plugins, Settings, Logs)
- [x] 2.4 Verify: server starts, renders base layout at `/`

## 3. Chi Router & View Endpoints

- [x] 3.1 Create `internal/httpserver/routes.go` — Chi route registration: `GET /` → library, `GET /view/{viewName}` → view handler
- [x] 3.2 Create `internal/httpserver/views.go` — view handler that renders Jet template (full page for direct nav, fragment for HTMX `HX-Request`)
- [x] 3.3 Implement library view: `GET /view/library` → render `library.jet` with manga list from bridge
- [x] 3.4 Implement search view: `GET /view/search?q=...` → render `search.jet` with search results
- [x] 3.5 Implement detail view: `GET /view/manga/{pluginID}/{mangaID}` → render `detail.jet` with manga + chapters
- [x] 3.6 Implement plugins view: `GET /view/plugins` → render `plugins.jet` with plugin list
- [x] 3.7 Implement settings view: `GET /view/settings` → render `settings.jet` with config values
- [x] 3.8 Implement logs view: `GET /view/logs` → render `logs.jet` with log buffer
- [x] 3.9 Verify: navigate to each view in browser, HTMX partial swap works

## 4. HTMX Action Endpoints

- [x] 4.1 Implement `POST /action/install-plugin` — multipart form upload, install plugin, return updated plugin list fragment
- [x] 4.2 Implement `POST /action/toggle-plugin/{pluginID}` — toggle active state, return updated plugin card
- [x] 4.3 Implement `POST /action/toggle-library/{pluginID}/{mangaID}` — add/remove from library, return updated card
- [x] 4.4 Implement `POST /action/sync` — sync library, return updated library view
- [x] 4.5 Implement `POST /action/set-chapter-progress` — record reading progress, return updated chapter list
- [x] 4.6 Implement `POST /action/save-settings` — save config, return updated settings view
- [x] 4.7 Verify: each action works via HTMX form submission, fragments swap correctly

## 5. Image Endpoint

- [x] 5.1 Implement `GET /image` — query params (pluginID, url, mangaID, chapterID), proxy via hostnet, return binary bytes
- [x] 5.2 Set correct `Content-Type` header based on image detection (JPEG/PNG/WebP/GIF)
- [x] 5.3 Serve from disk cache when available (per-plugin/manga/chapter layout)
- [x] 5.4 Verify: `<img src="/image?pluginID=mangadex&url=...">` renders in browser

## 6. Reader View (Canvas + Vanilla JS)

- [x] 6.1 Create `templates/views/reader.jet` — canvas element + controls (back, page counter, view modes, zoom)
- [x] 6.2 Port canvas rendering logic from `lib/reader.js` to inline `<script>` in reader.jet template
- [x] 6.3 Pass page list data from Go to JavaScript via Jet template variables (JSON in `<script>` tag)
- [x] 6.4 Implement chapter navigation via HTMX: next/prev chapter → `hx-get="/view/read/{pid}/{mid}/{cid}"` → swap reader content
- [x] 6.5 Preserve all reader features: zoom/pan/drag, fit-width/fit-height/original, keyboard nav, progress tracking, error panel, spinner, RTL/LTR
- [x] 6.6 Verify: reader loads, pages display, zoom/pan works, chapter navigation works

## 7. WebSocket Log Streaming

- [x] 7.1 Implement `GET /api/logs/ws` WebSocket endpoint using `golang.org/x/net/websocket`
- [x] 7.2 Implement `GET /api/logs` HTTP endpoint returning current log buffer as JSON array
- [x] 7.3 Wire logger ring buffer to broadcast new entries to all connected WebSocket clients
- [x] 7.4 Create `templates/views/logs.jet` — log viewer with WebSocket connection + auto-scroll
- [x] 7.5 Verify: open logs view, see live log entries streaming in real-time

## 8. Remove Wails & Alpine.js

- [x] 8.1 Remove `github.com/wailsapp/wails/v3` from go.mod and all Go imports
- [x] 8.2 Remove Wails-specific build tags (`-tags gtk3,production`) from Makefile
- [x] 8.3 Set `CGO_ENABLED=0` in Makefile build targets
- [x] 8.4 Remove `wails.json` if present
- [x] 8.5 Remove Alpine.js from frontend (`lib/alpine.min.js`)
- [x] 8.6 Remove old Alpine.js views (`lib/views/`, `lib/state.js`, `lib/bindings.js`, `lib/reader.js`, `lib/utils.js`, `lib/format.js`, `lib/readhash.js`)
- [x] 8.7 Remove old `app.js` entry point
- [x] 8.8 Verify: `make build` produces a pure Go binary without CGO

## 9. Makefile & Build Updates

- [x] 9.1 Update `make build` to use `CGO_ENABLED=0` and remove gtk3 tags
- [x] 9.2 Update `make dev` to build + auto-open browser
- [x] 9.3 Update `make run` to pass `--open` flag
- [x] 9.4 Add `make open` target that opens `http://localhost:PORT` in browser
- [x] 9.5 Update `make check` to remove CGO-dependent linting
- [x] 9.6 Verify: `make build && make run` starts server and opens browser

## 10. End-to-End Verification

- [x] 10.1 Test library view: manga list loads, covers display, add/remove from library works
- [x] 10.2 Test search: search results appear, add to library works
- [x] 10.3 Test detail: manga details + chapter list loads, chapter progress tracking works
- [x] 10.4 Test reader: chapter loads, pages display, canvas zoom/pan works, toolbar visible, chapter navigation works
- [x] 10.5 Test plugins: plugin list loads, install/toggle works
- [x] 10.6 Test settings: config loads/saves, reload works
- [x] 10.7 Test logs: live log streaming works, clear works
- [x] 10.8 Verify: `go test ./...` passes, `make check` passes
- [x] 10.9 Verify: cross-compile `GOOS=windows GOARCH=amd64 go build` works

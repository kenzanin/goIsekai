## Context

goIsekai currently uses Wails v3 (`github.com/wailsapp/wails/v3`) as its desktop UI layer, rendering the frontend in a webkit2gtk-4.1 webview with Alpine.js for client-side interactivity. The frontend communicates with Go backend via Wails `Call.ByName` bindings returning JSON. This architecture causes persistent WebKit2GTK GPU compositing bugs (canvas occlusion with `backdrop-filter`) that are unfixable at the application level. The CGO dependency chain (webkit2gtk-dev, gtk3-dev) limits cross-platform distribution.

The existing backend architecture (bridge service, plugin manager, host network, database) is well-structured and does not need to change — only the wiring layer between frontend and backend needs replacement.

## Goals / Non-Goals

**Goals:**
- Replace Wails desktop window with a Chi HTTP server serving Jet-rendered HTML to the user's browser
- Use HTMX for client-side interactivity (replaces Alpine.js)
- Use CloudyKit/jet for server-side template rendering (replaces client-side JSON + DOM manipulation)
- Maintain identical frontend functionality (library, search, reader, plugins, settings, logs)
- Achieve `CGO_ENABLED=0` pure Go binary
- Preserve the existing backend architecture (bridge, pluginmanager, hostnet, database, config, logger)

**Non-Goals:**
- Redesigning the frontend UI/UX (Tailwind CSS stays as-is)
- Adding authentication or HTTPS (localhost-only by default, configurable via `host` config/flag)
- Changing the plugin ABI or WASM runtime
- Adding a reverse proxy or production deployment setup

## Decisions

### Decision 1: Chi HTTP router (`github.com/go-chi/chi/v5`)

**Choice**: Chi router

**Alternatives considered**:
- `net/http` stdlib (Go 1.22+) — sufficient but lacks middleware stack
- `gin` — heavier, custom context not stdlib-compatible
- `echo` — opinionated, heavier

**Rationale**: Chi is lightweight (~10KB), 100% compatible with `http.Handler`, has excellent built-in middleware (Logger, Recoverer, Compress, CORS), and supports route grouping (`r.Route("/view", ...)`). It's the de facto standard for Go HTTP servers that want clean routing without framework lock-in.

### Decision 2: CloudyKit/jet template engine (`github.com/CloudyKit/jet/v6`)

**Choice**: Jet templates for server-side HTML rendering

**Alternatives considered**:
- `html/template` (stdlib) — no template inheritance, verbose, no custom expressions
- `templ` — requires code generation step, adds build complexity
- `Pongo2` — Django-style but less maintained

**Rationale**: Jet provides template inheritance (extends/block/yield), C-like expressions, auto-escaping, pre-parsed templates with low allocations, and custom functions. It's fast, well-maintained (1401 stars), and integrates cleanly with `http.ResponseWriter`. Template inheritance means a single base layout with per-view blocks — exactly what HTMX partial rendering needs.

### Decision 3: HTMX for client-side interactivity (`htmx.org` 2.x)

**Choice**: HTMX (16KB) replaces Alpine.js (44KB)

**Alternatives considered**:
- Keep Alpine.js — client-side state management conflicts with server-rendered HTML
- Vanilla JS only — too much boilerplate for form submissions, partial updates
- Unpoly — similar to HTMX but less ecosystem support

**Rationale**: HTMX's model (HTML fragments from server, swap into DOM) is a natural fit with server-side Jet rendering. The server owns the state, HTMX handles the transport. This eliminates the JSON API layer entirely — no `fetch()`, no JSON serialization, no client-side state management. HTMX attributes (`hx-get`, `hx-post`, `hx-swap`, `hx-target`) declaratively replace all Alpine.js `x-data`/`x-on`/`x-show` patterns.

**Exception**: The reader's canvas zoom/pan/drag remains vanilla JavaScript — HTMX cannot handle canvas interactions. The reader view is a Jet template that includes a `<script>` block with the canvas rendering logic.

### Decision 4: Template structure with Jet inheritance

**Choice**: Jet template inheritance pattern:
```
templates/
  layouts/
    base.jet          ← extends: <html>, <head>, <body>, yield "content"
  views/
    library.jet       ← extends base, block "content": manga grid
    search.jet        ← extends base, block "content": search results
    detail.jet        ← extends base, block "content": manga detail + chapters
    reader.jet        ← extends base, block "content": canvas + controls
    plugins.jet       ← extends base, block "content": plugin list
    settings.jet      ← extends base, block "content": settings form
    logs.jet          ← extends base, block "content": log viewer
  partials/
    manga-card.jet    ← reusable manga card component
    plugin-card.jet   ← reusable plugin card component
    chapter-list.jet  ← reusable chapter list component
    nav.jet           ← navigation bar
```

**Rationale**: HTMX requests with `HX-Request: true` header get the view fragment only (no layout). Direct browser navigation gets the full page (layout + view). Jet's `extends` and `block` directives handle this cleanly.

### Decision 5: Binary image endpoint separate from HTML

**Choice**: `GET /image` returns raw bytes with correct `Content-Type`.

**Rationale**: Images are binary data — serving them through Jet templates would be wasteful. The endpoint accepts query parameters and returns bytes directly. HTMX `hx-attr` or plain `<img src="/image?...">` handles image display.

### Decision 6: WebSocket for live log streaming

**Choice**: `golang.org/x/net/websocket` for the log WebSocket endpoint.

**Alternatives considered**:
- `nhooyr.io/websocket` — more modern API but heavier dependency
- HTMX SSE extension — simpler but SSE is one-directional
- HTMX polling (`hx-trigger="every 2s"`) — simplest but adds latency

**Rationale**: `golang.org/x/net/websocket` is lightweight and sufficient for a unidirectional log stream. The logs view uses HTMX polling as a fallback for browsers that don't support WebSocket.

### Decision 7: Keep `go:embed` for static files and templates

**Choice**: Continue using `go:embed` to embed frontend static files AND Jet templates.

**Rationale**: Single-binary distribution model preserved. Jet templates are loaded from the embedded filesystem at startup. Static files (Tailwind CSS, HTMX JS) served via `http.FileServer`.

## Risks / Trade-offs

**[Risk] HTMX + canvas reader hybrid complexity** → Mitigation: the reader is the only view that uses vanilla JS; all other views are pure HTMX. The reader Jet template includes a `<script>` block that initializes the canvas renderer with page data passed from the server.

**[Risk] Jet template compilation errors at runtime** → Mitigation: Jet pre-compiles templates at startup; errors surface immediately on boot, not at request time. Use `SetDevelopmentMode(true)` during development for hot-reload.

**[Risk] HTMX partial rendering requires careful fragment structure** → Mitigation: each view returns a single root element that HTMX swaps. Use `hx-swap="innerHTML"` targeting a `#content` container.

**[Risk] Browser not available on headless servers** → Mitigation: the server is localhost-only by default; headless use is API-only (no UI). Document this.

**[Risk] CORS issues if frontend is served from a different origin** → Mitigation: frontend and API are on the same origin (same port), so CORS is not needed.

**[Risk] Binding to 0.0.0.0 exposes the server on LAN** → Mitigation: default is `127.0.0.1` (localhost only). User must explicitly set `host = 0.0.0.0` in config or `--host 0.0.0.0` CLI flag to expose on LAN. Document security implications.

**[Risk] WebSocket connection lifecycle (reconnect on browser tab refresh)** → Mitigation: frontend auto-reconnects on `close` event with exponential backoff.

## Migration Plan

1. Add Chi router + Jet template engine + HTMX to dependencies
2. Create Jet template structure (base layout + view templates + partials)
3. Implement Chi routes for views (GET /view/*) and actions (POST /action/*)
4. Implement binary image endpoint (GET /image)
5. Implement WebSocket log endpoint (GET /api/logs/ws)
6. Rewrite frontend from Alpine.js to HTMX + Jet templates
7. Keep canvas reader as vanilla JS in a Jet template
8. Remove Wails dependency and CGO requirements
9. Update Makefile and build configuration
10. Test end-to-end: library, search, reader, plugins, settings, logs

## Open Questions

- ~~Should the server support `--host` flag for binding to `0.0.0.0` (LAN access)?~~ **Resolved**: Yes — `host` config key in `goisekai.ini` + `--host` CLI flag. Default `127.0.0.1` (localhost only). CLI flag overrides config.

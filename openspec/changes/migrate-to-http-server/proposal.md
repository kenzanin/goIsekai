## Why

goIsekai currently uses Wails v3 (webkit2gtk-4.1) as its desktop UI layer. This causes persistent rendering bugs — most recently, HTML5 `<canvas>` elements occlude sibling toolbar elements due to WebKit2GTK GPU compositing issues with `backdrop-filter`. These bugs are unfixable at the application level because they stem from the WebKit2GTK compositor itself. Additionally, the Wails/CGO dependency chain requires webkit2gtk-dev, gtk3-dev, and produces a 33MB binary that only runs on Linux with the correct GTK/WebKit libraries installed.

Migrating to a CLI HTTP server eliminates all WebKit/GTK dependencies, enables cross-platform distribution (any OS with a browser), and provides a better development experience (Chrome DevTools, standard web debugging).

## What Changes

- **BREAKING**: Remove Wails v3 desktop window — the app no longer opens its own window
- **BREAKING**: Replace Wails `Call.ByName` bridge binding with HTMX-driven HTML fragment endpoints
- **BREAKING**: Replace Alpine.js client-side state management with server-side Jet template rendering
- **BREAKING**: Replace Wails `[]byte` → base64 marshaling with direct binary HTTP responses for image data
- Add `go-chi/chi` HTTP router with middleware (Logger, Recoverer, Compress)
- Add `CloudyKit/jet` template engine for server-side HTML rendering
- Add HTMX (16KB) for client-side interactivity (replaces Alpine.js 44KB)
- Add WebSocket endpoint for live log streaming (replaces Wails `Log()` binding)
- Add `--port` CLI flag and `port` config key for HTTP server port (default: 8080)
- Add `--host` CLI flag and `host` config key for bind address (default: 127.0.0.1)
- Add `--open` CLI flag to auto-open browser on startup
- Update Makefile: `CGO_ENABLED=0`, remove `-tags gtk3,production`, add `open` target
- Update build: pure Go binary, no CGO required

## Capabilities

### New Capabilities
- `http-server`: Chi HTTP server serving Jet-rendered HTML fragments + HTMX-driven UI + binary image endpoint + WebSocket log streaming

### Modified Capabilities
- `bridge`: Bridge service methods exposed via HTMX HTML fragment endpoints instead of Wails bindings; Jet templates render each view; image endpoint returns binary instead of base64
- `host-network`: No spec-level changes (tls-client proxy unchanged, only wiring changes)
- `plugin-abi`: No spec-level changes (WASM ABI unchanged)
- `plugin-runtime`: No spec-level changes (wazero runtime unchanged)
- `storage`: No spec-level changes (database unchanged)

## Impact

- **Code**: `cmd/goisekai/main.go` (rewrite), `internal/httpserver/` (new), `internal/templates/` (new Jet templates), `cmd/goisekai/frontend/` (rewrite: Alpine.js → HTMX + Jet), `Makefile` (update)
- **Dependencies**: Remove `github.com/wailsapp/wails/v3`, add `github.com/go-chi/chi/v5`, `github.com/CloudyKit/jet/v6`, `htmx.org`
- **Build**: `CGO_ENABLED=0` — pure Go binary, cross-compile to any OS without CGO toolchain
- **Binary size**: ~20MB (down from ~33MB)
- **Runtime**: Requires browser (Chrome/Firefox/Edge) instead of bundled WebKit
- **Config**: Add `port` key to `goisekai.ini`

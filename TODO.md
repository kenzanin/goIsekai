# TODO — goIsekai Gaps & Improvements

Items from honest project review (2026-09-03). Each item is a candidate for an OpenSpec change.

---

## 🔴 High Priority

### 1. httpserver Test Coverage (currently 6.8%)
All user-facing logic lives here — handlers, views, actions, API, sandbox, reader-data — but almost zero automated tests. Every bug so far (qrm alias trap, chapter order, status normalization) was caught by manual browser testing.

**Scope:**
- `api.go` — 11 endpoints, 0 handler tests (only middleware test exists)
- `views.go` — viewMangaDetail, viewSearch, viewHistory (render + data assembly)
- `actions.go` — all POST action handlers (toggle-library, mark-read, mark-read-range, reset-progress, export-cbz, clear-cache, sync, save-settings)
- `reader.go` — readerData assembly (chapter list fallback, read-ahead prefetch)
- `sandbox.go` — load/unload/reload/call endpoints

**Approach:** httptest.NewServer + chi router, mock bridge service interface, table-driven tests per handler.

---

### 2. Config File Persistence
Settings (CDP engine, API key, port, host, log level, user-agent, referer) are CLI-flag-only. Restart = lost. `goisekai.ini` struct exists in config.go but has no read/write logic.

**Scope:**
- Read `goisekai.ini` (or `config.yaml`) on startup if it exists
- CLI flags override file values (flag > file > default)
- Settings page "Save" writes config file
- Hot-reload for safe subset (log level, user-agent) without restart

---

## 🟡 Medium Priority

### 3. Frontend Error Boundary
Reader.js (~360 lines vanilla JS) has no try-catch on critical paths. If `/api/reader-data` returns error or corrupt JSON, user sees blank canvas with no feedback. Search/detail pages use HTMX which has its own error handling, but the reader is pure fetch + canvas.

**Scope:**
- try-catch around all fetch calls in reader.js
- Error toast / overlay on reader canvas (network error, parse error, plugin error)
- Consistent error display across HTMX views (htmx:responseError event handler)

---

### 4. Plugin Error Reporting in UI
Plugin failure → host logs error → user sees "No chapters yet" or empty page. No per-plugin error state, no retry button, inconsistent challenge detection feedback.

**Scope:**
- Per-plugin error state in search/detail views (spinner → error message with reason)
- Retry button on transient errors (network timeout, 502)
- Consistent ChallengeError handling across all views (some handle it, some don't)
- Plugin health indicator on plugins page (last successful call, error count)

---

### 5. CDP Fallback Chain
`-cdpEngine obscura` is optional. Without it, challenge-blocked sites show a "paste cookies" banner. No automatic fallback: tls-client → detect 403/503+CF markers → try CDP → cookies back to jar → retry. This pattern is proven in Suwayomi/Mihon.

**Scope:**
- Detect challenge response (403/503 + CF markers) in bridge layer
- Automatic CDP fallback when engine is configured
- Cookie jar injection after CDP solves challenge
- Retry original request with new cookies
- Configurable: auto-fallback vs manual-only

---

### 6. Graceful Shutdown
Server is killed via `pkill`. No signal handler for SIGTERM/SIGINT to drain in-flight requests, close DB connections, flush logs, and stop plugin runtimes cleanly.

**Scope:**
- `signal.NotifyContext` on SIGTERM/SIGINT
- `http.Server.Shutdown(ctx)` with timeout
- DB close, plugin manager close, log flush
- PID file for clean stop (`goisekai stop`)

---

### 7. HTTP Access Logging
No request logging middleware. Cannot diagnose slow requests, error rates, or traffic patterns.

**Scope:**
- Middleware: method, path, status, latency, size
- Structured logging (slog) integration
- Skip health-check and static asset noise
- Optional request ID for tracing

---

### 8. Unified Plugin Build
`make build` at root does not rebuild WASM plugins. User must manually `cd examples/wasm/X && make build && cp dist/X.wasm ../../app_data/plugins/`. Easy to forget, leads to stale plugin binaries.

**Scope:**
- Root `make build-plugins` target that builds all WASM plugins
- `make all` = build-plugins + build host
- Optional: `make install-plugins` copies to app_data/plugins/
- CI-friendly: exit non-zero on any plugin build failure

---

## 🟢 Low Priority

### 9. Reader Keyboard Shortcuts
Reader has no keyboard navigation. Arrow keys for prev/next page, space for next page, escape to go back to detail — standard manga reader UX.

---

### 10. Plugin Build Size Optimization
Binary + embedded assets + 4 WASM plugins = large. No compression or lazy-load for WASM files (each ~4-5MB).

**Scope:**
- `go:embed` with gzip for WASM files (decompress on load)
- Or: lazy-load WASM on first plugin call instead of startup
- Binary size audit (strip debug symbols, UPX)

---

## Deferred to v1.0
- **Database migration versioning** — intentionally deferred until stable schema

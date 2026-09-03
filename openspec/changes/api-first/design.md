# Design: API-First

## Context
The server (`internal/httpserver`) already splits cleanly: `routes.go` wires chi, `views.go`/`actions.go`/`reader.go` are thin handlers over `internal/bridge` AppService methods. JSON exists only at `/api/reader-data`, sandbox, and CDP endpoints. Auth today is security-by-loopback (default bind `127.0.0.1:8080`). See proposal.md — Why.

Constraints that shape the approach:
- chi v5 + Jet templates are settled (project rules #1998/#1999) — no new framework
- `GET /image` must stay header-flexible (Referer forwarding is load-bearing for mangafire) and header-less (browser `<img>` tags can't send `X-API-Key`)
- Plugin params must be read via the `param()` helper (PathUnescape) per constraint #2092/#2095
- Error paths in current handlers use `http.Error` with text bodies — API handlers must not reuse that shape

## Goals / Non-Goals
**Goals:**
- One JSON endpoint per view endpoint, sharing bridge service calls
- Uniform JSON error envelope + status codes
- Optional API-key middleware scoped to `/api/*` (exempt `GET /image`)
- Keep every existing route byte-compatible in behavior
- Concise `docs/API.md` for external client authors

**Non-Goals:**
- No OpenAPI spec/codegen, no `/v1/` versioning (add when real external consumers exist)
- No per-user accounts, sessions, tokens, or rate limiting
- No CORS policy by default (custom UIs are expected to be server-proxied or same-origin; revisit on demand)
- No changes to view handlers, templates, or the plugin ABI

## Decisions

**D1 — Parallel handlers, not shared handler with content negotiation.**
Each API endpoint is its own small handler function calling the same service methods. Alternative (one handler doing `Accept: application/json` branching) was rejected: it entangles template rendering with JSON encoding, makes the error paths divergent, and produces longer functions. Two thin handlers per resource beats one clever one. Duplication is bounded to the ~10 lines of param/decode glue per endpoint; the domain logic stays single-sourced in bridge.

**D2 — One file, `internal/httpserver/api.go`.**
All JSON handlers + envelope helpers (`writeErr(w, status, msg)`, `writeJSON(w, status, v)`) live in one new file. Route wiring stays in `routes.go` under a clearly grouped `/api/` block. Keeps the JSON surface greppable in one place.

**D3 — API key via chi middleware on the `/api` subgroup, image exempt by construction.**
`-apiKey` flag (empty default) / `api_key` ini key feed one value into the server struct. Middleware: if `apiKey == ""` → `next`; else compare `X-API-Key` header constant-time (`subtle.ConstantTimeCompare`). `GET /image` stays mounted outside the `/api` group (it already is), so it inherits no middleware — no per-route exemption list needed. Constant-time compare chosen because keys can be the only secret guarding plugin hot-load (sandbox) — cheap insurance.

**D4 — 401/403 distinction not used.**
Always `401 {"error":"unauthorized"}` for bad/missing keys regardless of whether a key is configured (when configured). Avoids leaking "key exists" vs "key wrong" to probes. Documented in API.md.

**D5 — Envelope only on errors; payloads are bare objects/arrays.**
`{"error": "..."}` on failure; success returns the data directly (e.g., `GET /api/library` → `[...]`). No `{"data": ...}` wrapper — it's noise for a personal-app API and the reader-data endpoint already sets the precedent of bare payloads. Field naming: snake_case to match reader-data's existing keys (`pluginID` is camel there — reader-data keeps its shape for backward compat; new endpoints standardize on snake_case. This inconsistency is documented, not migrated).

**D6 — Pagination in JSON mirrors view semantics.**
`GET /api/search` takes `pluginID`, `q`, `page` (1-based); response `{"results": [...], "has_next": bool, "page": N}` computed identically to the view handler's slice/HasNext logic (host-side, `search_page_size`-aware).

**D7 — Actions over POST with JSON responses.**
`POST /api/library/toggle/{pluginID}/{mangaID}`, `POST /api/chapters/read/{pluginID}/{mangaID}/{chapterID}`, `POST /api/progress/{pluginID}/{mangaID}/{chapterID}` (body: `{"page": N}`, `{"total_pages": N}` optionally). Returns the new state (`{"in_library": true}`) instead of 303 — API clients get data, views keep redirects.

## Risks / Trade-offs
- **Open API when key unset**: if a user binds `0.0.0.0` without `-apiKey`, `/api/*` (including sandbox hot-load) is LAN-open. Mitigation: startup WARN log when server binds non-loopback and `apiKey` is empty; API.md states it first.
- **Two handlers per resource** doubles the param-glue lines vs content negotiation. Accepted: each is <15 lines and independently testable; revisit only if endpoint count balloons.
- **snake_case vs reader-data's camelCase**: cosmetic inconsistency. Accepted to avoid a breaking change to reader-data consumers (the in-app reader JS).
- **No CORS**: a browser-based third-party UI on another origin can't call the API directly. Deferred (ponytail: add when a real consumer exists — it's ~10 lines of chi middleware).

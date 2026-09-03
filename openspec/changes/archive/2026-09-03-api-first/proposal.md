# Proposal: API-First Architecture

## Why
goIsekai's UI is server-rendered (Jet templates), so any custom UI a third party wants to build has no machine-facing surface to consume — the only JSON today is `/api/reader-data` and the sandbox. The bridge service layer already holds all domain logic behind thin view handlers, so exposing the same data as JSON is a small, non-breaking step that opens the app to external clients (alternative frontends, scripts, mobile wrappers).

## What Changes
- Add JSON API endpoints under `/api/` that mirror every view endpoint, sharing the same `bridge` service calls (no logic duplication)
- Standardize API error responses as `{"error": "..."}` with correct HTTP status codes (400 missing params, 404 unknown resource, 502 upstream plugin failure)
- Add optional API-key auth: `-apiKey` CLI flag / `api_key` config key; when set, all `/api/*` routes (except `GET /image`) require the `X-API-Key` header. When unset, `/api/*` behaves as today (loopback trust)
- `GET /image` stays header-passthrough (Referer forwarding) and API-key-exempt so `<img>` tags work from any UI
- Keep ALL existing `/view/*` and `/action/*` routes unchanged — the server-rendered UI keeps working as-is (non-breaking)
- Document the endpoints in `docs/API.md` (concise markdown, no OpenAPI codegen at this stage)

## Capabilities

### New Capabilities
- `http-api`: JSON API surface under `/api/` — endpoint inventory, response/error envelope, and API-key middleware semantics

### Modified Capabilities
- `http-server`: add the requirement that `/api/*` routes mount an optional API-key middleware and that JSON and view handlers share one service layer (no behavior change to existing view routes)

## Impact
- **Code**: new `internal/httpserver/api.go` (JSON handlers + envelope helpers), middleware in `server.go` route wiring, `-apiKey` flag in `cmd/goisekai`; bridge service untouched
- **Docs**: new `docs/API.md`
- **Risk**: low — additive routes only; the auth default (unset key = open `/api/*` on whatever interface the server binds) must be documented clearly; users exposing beyond loopback must set `-apiKey`
- **Out of scope**: OpenAPI spec/codegen, versioned `/v1/` prefixes, per-user auth, rate limiting, changing any existing view/action route

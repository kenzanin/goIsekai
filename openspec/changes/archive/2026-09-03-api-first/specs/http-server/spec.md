# Delta: HTTP Server — API-first middleware and shared service layer

Extends the HTTP server capability so the JSON API under `/api/` is gated by an optional API key and shares the bridge service layer with the view handlers, without changing any existing view/action behavior.

## ADDED Requirements

### Requirement: Optional API-key middleware on /api routes
When an API key is configured (`-apiKey` flag or `api_key` config key), the server SHALL mount an authentication middleware on `/api/*` (excluding `GET /image`) that rejects requests lacking a matching `X-API-Key` header with `401` and the JSON error envelope. When no key is configured, the middleware SHALL pass all requests through unchanged.

#### Scenario: Configured key rejects bad header
- **WHEN** an API key is configured and a request hits `/api/library` without `X-API-Key`
- **THEN** the server responds `401` with `{"error": "unauthorized"}` before any handler logic runs

#### Scenario: Unconfigured key passes through
- **WHEN** no API key is configured
- **THEN** requests to `/api/*` reach their handlers regardless of headers

### Requirement: JSON and view handlers share one service layer
Each `/api/*` handler SHALL obtain its data by calling the same bridge service methods as its corresponding view handler. Introducing a JSON endpoint SHALL NOT duplicate domain logic or query code.

#### Scenario: Data parity between view and API
- **WHEN** the library state changes and a client compares `GET /view/library` (parsed HTML) with `GET /api/library`
- **THEN** both reflect the same underlying library entries from the same service call

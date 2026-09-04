## Purpose

Provides structured HTTP request logging for diagnosing slow requests, error rates, and traffic patterns.

## ADDED Requirements

### Requirement: Request logging middleware
Every HTTP request SHALL be logged with: method, path, status code, response size, and latency (ms). Logs SHALL use structured slog format.

#### Scenario: Normal request logged
- **WHEN** a request completes (success or error)
- **THEN** a structured log entry is emitted with method, path, status, size, and latency

#### Scenario: Error requests highlighted
- **WHEN** a request returns 4xx or 5xx
- **THEN** the log entry includes the status at WARN or ERROR level respectively

### Requirement: Noise filtering
Health check endpoints (`/health`, `/ready`) and static asset requests (`.js`, `.css`, `.webp`, `.br`) SHALL be excluded from logging by default.

#### Scenario: Health check not logged
- **WHEN** `/health` is requested
- **THEN** no log entry is emitted

#### Scenario: Static asset not logged
- **WHEN** `/static/app.js` is requested
- **THEN** no log entry is emitted

### Requirement: Request ID
Each request SHALL have a unique request ID (UUID or short hash). If the client sends `X-Request-Id`, it SHALL be preserved; otherwise generated server-side. The ID SHALL appear in the log entry and response header.

#### Scenario: Client provides request ID
- **WHEN** a request includes `X-Request-Id: abc123`
- **THEN** the log entry and response header use `abc123`

#### Scenario: Server generates request ID
- **WHEN** a request has no `X-Request-Id` header
- **THEN** the server generates a short ID and includes it in log and response
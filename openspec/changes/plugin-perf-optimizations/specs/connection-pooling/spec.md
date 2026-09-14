## Purpose

Maintain persistent HTTP connections to plugin target sites so subsequent requests reuse existing TCP+TLS sessions instead of performing fresh handshakes each time.

## ADDED Requirements

### Requirement: HTTP connection pool
The hostnet proxy SHALL maintain a pool of persistent HTTP connections per target host. Connections in the pool SHALL be reused for subsequent requests to the same host when possible.

#### Scenario: Connection reused for same host
- **WHEN** a plugin makes an HTTP request to a host with an existing idle connection
- **THEN** the proxy reuses the existing connection instead of creating a new one

#### Scenario: New connection created when pool empty
- **WHEN** a plugin makes an HTTP request to a host with no idle connections
- **THEN** the proxy creates a new connection, adds it to the pool, and uses it

#### Scenario: Connection closed when pool limit reached
- **WHEN** the connection pool for a host reaches its maximum size
- **THEN** the proxy closes the oldest idle connection before adding a new one

### Requirement: Per-host connection limits
The proxy SHALL enforce configurable maximum concurrent connections per target host. The default limit SHALL be 6 connections per host, matching browser behavior.

#### Scenario: Concurrent requests within limit
- **WHEN** multiple concurrent requests target the same host and the count is below the limit
- **THEN** each request gets its own connection from the pool

#### Scenario: Concurrent requests exceed limit
- **WHEN** concurrent requests to the same host exceed the connection limit
- **THEN** excess requests queue until a connection becomes available

### Requirement: Connection keep-alive
The proxy SHALL honor HTTP keep-alive headers and maintain connections in the pool for as long as the server allows. Connections SHALL be closed when the server signals closure or when idle timeout expires.

#### Scenario: Server sends keep-alive
- **WHEN** the target server responds with keep-alive headers
- **THEN** the proxy keeps the connection in the pool for reuse

#### Scenario: Server signals connection close
- **WHEN** the target server responds with `Connection: close`
- **THEN** the proxy closes the connection after the response and does not pool it

### Requirement: Idle connection timeout
The proxy SHALL close idle connections after a configurable timeout (default: 90 seconds) to prevent resource leaks.

#### Scenario: Idle timeout expires
- **WHEN** a pooled connection has been idle for longer than the timeout
- **THEN** the proxy closes the connection and removes it from the pool

#### Scenario: Connection reused before timeout
- **WHEN** a pooled connection is reused before the idle timeout
- **THEN** the timeout resets and the connection remains in the pool

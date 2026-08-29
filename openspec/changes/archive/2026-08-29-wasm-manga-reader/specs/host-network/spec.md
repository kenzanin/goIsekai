## Purpose

Provides sandboxed HTTP access so plugins fetch remote manga content through the host instead of opening network sockets themselves, letting the host enforce headers and session state centrally.

## ADDED Requirements

### Requirement: Socket access forbidden
Plugins SHALL NOT directly initialize socket connections; all network access SHALL go through a host-imported request function.

#### Scenario: Plugin attempts direct network access
- **WHEN** a plugin tries to open a socket or dial a connection directly
- **THEN** the host denies the attempt and surfaces an error to the caller

### Requirement: Host HTTP request function
The host SHALL expose a `host_http_request` import that accepts `request_payload_json` and returns `response_payload_json`.

#### Scenario: Plugin performs a request
- **WHEN** a plugin calls `host_http_request` with a request payload containing method, URL, headers, and body
- **THEN** the host performs the HTTP request and returns the response status, headers, and body as JSON

### Requirement: Standard header injection
The host SHALL automatically inject standard headers (`User-Agent`, `Accept-Language`, and where configured `Referer`) into outbound requests.

#### Scenario: Request without explicit headers
- **WHEN** a plugin issues a request that omits standard headers
- **THEN** the host injects the default `User-Agent`, `Accept-Language`, and configured `Referer` before sending

### Requirement: Per-page header override
Page-level custom headers SHALL override host defaults for that page's image fetch.

#### Scenario: Page-specific referer
- **WHEN** a page's `headers` map specifies a custom `Referer`
- **THEN** the host uses that Referer instead of the default when fetching the page image

### Requirement: Cookie persistence
The host SHALL persist cookies across plugin calls so authenticated or session-based sources keep working.

#### Scenario: Session-based source
- **WHEN** a plugin performs a login that sets session cookies
- **THEN** subsequent requests for that source include the persisted cookies automatically

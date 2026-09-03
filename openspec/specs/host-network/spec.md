# Host Network Specification

## Purpose

Provides sandboxed HTTP access so plugins fetch remote manga content through the host instead of opening network sockets themselves, letting the host enforce headers and session state centrally.

## Requirements

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

### Requirement: Automatic CDP fallback on challenge
When the host detects a challenge response (HTTP 403/503 with challenge markers) and a CDP engine is enabled, the host SHALL attempt to solve the challenge via the engine, inject the harvested cookies into the plugin's session, and retry the original request before surfacing a challenge error to the caller.

#### Scenario: Fallback succeeds and retry passes
- **WHEN** a plugin request returns a challenge response and the engine solves it
- **THEN** the host injects the harvested cookies and retries the request, returning the successful response to the caller

#### Scenario: Fallback disabled
- **WHEN** a challenge response is detected but the CDP engine is `off`
- **THEN** the host surfaces the challenge error immediately without attempting a solve

#### Scenario: Fallback fails
- **WHEN** the engine is enabled but the solve attempt times out or the retry returns another challenge
- **THEN** the host surfaces the original challenge error so the existing manual-verify flow can proceed

### Requirement: Cookie handoff into plugin session
Cookies harvested by the CDP engine SHALL be handed to the host's existing per-plugin verify-cookie mechanism so subsequent fast-path requests carry them, with the User-Agent bound to the browser identity that solved the challenge.

#### Scenario: Harvested cookies reused
- **WHEN** the engine solves a challenge and returns cookies plus the browser User-Agent
- **THEN** the host stores both via the existing verify-cookie path, so the next fast-path request for that plugin carries them

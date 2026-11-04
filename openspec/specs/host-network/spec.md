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

### Requirement: Plugin-declared User-Agent ladder
A plugin MAY declare an ordered `user_agents` list in its `plugin.json`. The host SHALL send the first entry as the `User-Agent` of every request that plugin makes, including image fetches the host makes on its behalf, until a block response is observed. The literal entry `default` SHALL stand for the host's configured User-Agent. Empty entries SHALL be ignored. A plugin that declares no list SHALL keep the host default.

#### Scenario: First entry is used from the first request
- **WHEN** a plugin declares `user_agents` as `["goIsekai/1.0", "default"]`
- **AND** it issues its first request
- **THEN** the request carries `User-Agent: goIsekai/1.0`

#### Scenario: Plugin without a list
- **WHEN** a plugin declares no `user_agents`
- **THEN** its requests carry the host's configured User-Agent

#### Scenario: Image fetches follow the plugin's list
- **WHEN** the host fetches a cover or page image for a plugin that declares `user_agents`
- **THEN** the fetch uses that plugin's current User-Agent, not the host default

### Requirement: User-Agent fallback on block
When a request is answered with a block response (a recognised challenge, a WAF block, or a non-image HTML response to an image request), the host SHALL retry it with the plugin's next untried `user_agents` entry before escalating to a browser engine. The first entry that yields a non-block response SHALL be remembered for that plugin for the life of the process, and SHALL be used for its subsequent requests. When every entry is blocked the host SHALL surface the challenge error.

#### Scenario: Second entry clears the block
- **WHEN** the first entry is blocked and the second returns a normal response
- **THEN** the host returns the second response to the caller
- **AND** later requests for that plugin start with the second entry

#### Scenario: Every entry blocked
- **WHEN** every declared entry returns a block response
- **THEN** the host surfaces the challenge error so the existing verify flow can proceed

#### Scenario: Resetting a plugin's profile
- **WHEN** the user resets a plugin's pinned TLS profile from the plugins page
- **THEN** the remembered User-Agent for that plugin is cleared as well

### Requirement: User-Agent precedence
The host SHALL resolve the `User-Agent` of a request in this order: the User-Agent seeded by a completed verification for that plugin, then a `User-Agent` header the plugin sets on that specific request, then the plugin's remembered or first declared entry, then the host default. The `Sec-CH-UA` hint SHALL follow the User-Agent actually sent and SHALL be omitted for a non-Chromium value.

#### Scenario: Verification clearance keeps its identity
- **WHEN** a plugin has cookies seeded from a verification together with a User-Agent
- **THEN** requests use that User-Agent even if the plugin declares `user_agents`

#### Scenario: Per-request header wins over the list
- **WHEN** a plugin sets a `User-Agent` header on one request
- **THEN** that request uses it and the declared list is not consulted

#### Scenario: Client hint follows the value
- **WHEN** the User-Agent sent is not Chromium-family
- **THEN** no `Sec-CH-UA` header is sent

### Requirement: Non-image response to an image request is a block
When a 2xx response to an image request has an HTML content type or a body that is not a decodable image, and the body is an HTML document, the host SHALL treat it as a block response rather than as corrupt image data. It SHALL NOT repeat the identical request, and it SHALL NOT cache the body.

#### Scenario: Edge interstitial served as 200
- **WHEN** an image URL answers 200 with an HTML verification page
- **THEN** the host moves to the next User-Agent entry or TLS profile instead of retrying the same request
- **AND** nothing is written to the image cache

#### Scenario: Genuinely corrupt image bytes
- **WHEN** a 2xx response is not HTML and does not decode as an image
- **THEN** the host keeps its existing corrupt-image retry behaviour

#### Scenario: Blocked image surfaces as a challenge
- **WHEN** every User-Agent entry and TLS profile returns the HTML interstitial
- **THEN** the image request fails with a challenge error, not an "invalid image data" error

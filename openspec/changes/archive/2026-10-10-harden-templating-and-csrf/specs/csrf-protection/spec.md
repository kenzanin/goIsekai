# Spec Delta

## Purpose

Protects the HTML form surface of the reader from Cross-Site Request Forgery by requiring a token that only same-origin pages can produce, so a third-party page the browser loads cannot drive state-changing actions.

## ADDED Requirements

### Requirement: Token issuance

The system SHALL mint a random CSRF secret once per process start and SHALL derive the request token from that secret. The token SHALL be stable for the lifetime of the process so a page rendered before a later request still validates.

#### Scenario: Token is stable across requests
- **WHEN** two pages are rendered at different times within one server run
- **THEN** both pages carry the same token value

#### Scenario: Token changes across a restart
- **WHEN** the server restarts
- **THEN** the token issued after the restart differs from the token issued before it

### Requirement: Token delivery to the page

Every rendered page SHALL carry the token in a machine-readable form so that both plain HTML form submissions and script-driven requests can present it.

#### Scenario: Token present in a full page render
- **WHEN** a view route renders a full page
- **THEN** the output contains the token in a meta element and the plain forms in that page carry it as a submitted field

#### Scenario: Token available to script-driven requests
- **WHEN** a script on the page issues a state-changing request
- **THEN** the token it read from the page can be sent back on that request without reloading the page

### Requirement: Enforcement on state-changing routes

The system SHALL reject a state-changing request to an HTML action route when the token is absent or does not match, and SHALL reject it before the handler performs any mutation. Enforcement SHALL cover every mutating method, not only POST. Read-only routes and the JSON API SHALL NOT be subject to this check.

#### Scenario: Missing token is rejected
- **WHEN** a mutating request arrives at an HTML action route without a token
- **THEN** the request is refused and the underlying action does not run

#### Scenario: Wrong token is rejected
- **WHEN** a mutating request arrives carrying a token that does not match the process token
- **THEN** the request is refused and the underlying action does not run

#### Scenario: Correct token proceeds
- **WHEN** a mutating request arrives carrying the current token
- **THEN** the action runs and behaves exactly as it did before protection was added

#### Scenario: Rejection is visible to the caller
- **WHEN** a mutating request is refused
- **THEN** the caller receives a client-error status and a body explaining that the request was rejected, and no partial mutation is left behind

#### Scenario: Read routes stay reachable
- **WHEN** a page route or a JSON API route is requested without a token
- **THEN** the route behaves as before and is not refused

### Requirement: Rejection is logged

The system SHALL record a refused request in the request log with enough detail to identify the route and the reason, so an operator can distinguish a blocked forgery attempt from a client bug.

#### Scenario: Blocked attempt is visible in logs
- **WHEN** a mutating request is refused for a token mismatch
- **THEN** the request log contains an entry naming the refused path and the reason
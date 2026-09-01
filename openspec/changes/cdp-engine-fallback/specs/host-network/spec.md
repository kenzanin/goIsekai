## Delta: Host Network — CDP engine fallback on challenge

Extends the host HTTP request flow so an anti-bot challenge is automatically solved by a browser engine and retried, instead of only surfacing a challenge error for manual verification.

## ADDED Requirements

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

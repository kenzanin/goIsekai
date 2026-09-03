## Purpose

Automatically detect challenge-blocked HTTP responses and fall back to CDP engine to solve challenges, injecting cookies back into the jar for retry — transparent to plugins.

## ADDED Requirements

### Requirement: Challenge detection
The host HTTP layer SHALL detect challenge responses by checking for HTTP 403/503 status codes combined with Cloudflare/Turnstile markers in the response body (e.g., "cf-challenge", "turnstile", "Just a moment...").

#### Scenario: Cloudflare challenge detected
- **WHEN** an HTTP request returns 403 with Cloudflare markers in the body
- **THEN** the host marks the request as challenge-blocked

#### Scenario: Normal 403 not treated as challenge
- **WHEN** an HTTP request returns 403 without challenge markers
- **THEN** the host treats it as a normal error (no fallback)

### Requirement: Automatic CDP fallback
When a challenge is detected and a CDP engine is configured, the host SHALL automatically attempt to solve the challenge via CDP, extract cookies, inject them into the plugin's cookie jar, and retry the original request.

#### Scenario: CDP solves challenge
- **WHEN** a challenge is detected and CDP engine is available
- **THEN** the host uses CDP to load the page, waits for challenge resolution, extracts cookies, injects into jar, and retries the request

#### Scenario: CDP not configured
- **WHEN** a challenge is detected but no CDP engine is configured
- **THEN** the host returns the challenge error to the plugin (existing behavior)

#### Scenario: CDP fails to solve
- **WHEN** CDP attempt times out or fails
- **THEN** the host returns the original challenge error

### Requirement: Configurable fallback mode
A config option SHALL control whether CDP fallback is automatic or manual-only.

#### Scenario: Auto mode
- **WHEN** `cdp_fallback = auto` in config
- **THEN** challenges trigger automatic CDP fallback

#### Scenario: Manual mode
- **WHEN** `cdp_fallback = manual` in config
- **THEN** challenges show the "paste cookies" banner (existing behavior)

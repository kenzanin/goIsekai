## Context
The bridge HTTP layer uses tls-client for all plugin requests. Challenge-blocked sites (403/503 + CF markers) currently show a manual "paste cookies" banner. The CDP engine (obscura) exists but is only used for explicit testing via sandbox endpoints.

## Goals
1. Detect challenge responses automatically
2. Fall back to CDP to solve challenges when configured
3. Inject solved cookies into plugin jar and retry
4. Configurable: auto vs manual fallback

## Decisions

### D1: Challenge detection in hostnet layer
Add a `isChallengeResponse(resp)` function in `internal/hostnet/` that checks status code (403/503) and scans body for known challenge markers. Called after every HTTP response in the plugin request path.

### D2: Fallback loop in bridge request path
The bridge's `doRequest` method wraps the normal request: if `isChallengeResponse` returns true and CDP is configured + auto mode, invoke CDP → extract cookies → inject jar → retry once. Max 1 retry to prevent loops.

### D3: Cookie extraction from CDP
After CDP loads the page and the challenge resolves, extract cookies from the CDP browser's cookie jar and inject them into the tls-client cookie jar for the target domain.

### D4: Config option
Add `cdp_fallback` to config: `auto` (default when CDP engine is set), `manual`, `off`. Stored in goisekai.ini.

## Risks
- CDP challenge solving may take 5-30s — acceptable for manga sites, not for high-frequency API calls
- Cookie injection may not work for all anti-bot systems (some use JS-generated tokens)
- Retry loop must be bounded (max 1 retry) to prevent infinite loops

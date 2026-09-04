## Why
`-cdpEngine obscura` is optional. Without it, challenge-blocked sites show a "paste cookies" banner. No automatic fallback chain exists: tls-client → detect 403/503+CF markers → try CDP → cookies back to jar → retry. This pattern is proven in Suwayomi/Mihon.

## What Changes
- Detect challenge response (403/503 + CF markers) in bridge layer
- Automatic CDP fallback when engine is configured
- Cookie jar injection after CDP solves challenge
- Retry original request with new cookies
- Configurable: auto-fallback vs manual-only

## Capabilities
### New Capabilities
- `cdp-fallback`: Automatic CDP fallback chain for challenge-blocked HTTP requests

### Modified Capabilities
- `host-network`: Add challenge detection and CDP fallback to the HTTP client layer

## Impact
- `internal/hostnet/` — challenge detection + fallback logic in HTTP client
- `internal/config/` — auto-fallback toggle setting
- `internal/httpserver/` — remove manual "paste cookies" banner when auto-fallback is active
- Plugin ABI unchanged — fallback is transparent to plugins

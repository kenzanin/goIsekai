## Why
Plugin failure silently shows "No chapters yet" or empty page. No per-plugin error state, no retry button, inconsistent challenge detection feedback across views.

## What Changes
- Per-plugin error state in search/detail views (spinner → error message with reason)
- Retry button on transient errors (network timeout, 502)
- Consistent ChallengeError handling across all views
- Plugin health indicator on plugins page (last successful call, error count)

## Capabilities
### New Capabilities
- `plugin-error-reporting`: Per-plugin error state, retry buttons, and health indicators in the UI

### Modified Capabilities
(none)

## Impact
- `internal/templates/views/search.jet`, `detail.jet` — error state display
- `internal/templates/views/plugins.jet` — health indicator
- `internal/httpserver/views.go` — pass error state to templates
- `internal/bridge/` — track per-plugin error state
- No plugin ABI changes
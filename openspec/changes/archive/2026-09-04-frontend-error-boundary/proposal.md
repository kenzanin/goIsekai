## Why
Reader.js (~360 lines vanilla JS) has no try-catch on critical paths. If `/api/reader-data` returns error or corrupt JSON, user sees blank canvas with no feedback. Search/detail pages use HTMX which has its own error handling, but the reader is pure fetch + canvas.

## What Changes
- Add try-catch around all fetch calls in reader.js
- Error toast/overlay on reader canvas (network error, parse error, plugin error)
- Consistent error display across HTMX views (htmx:responseError event handler)

## Capabilities
### New Capabilities
- `frontend-error-boundary`: Error handling and user-facing error display for reader canvas and HTMX views

### Modified Capabilities
(none)

## Impact
- `cmd/goisekai/frontend/lib/reader.js` — try-catch around fetch calls, error overlay
- `cmd/goisekai/frontend/lib/app.js` or base layout — htmx:responseError handler
- `internal/templates/layouts/base.jet` — error toast HTML element
- No backend changes

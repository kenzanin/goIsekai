## Context
Reader.js uses vanilla JS fetch + canvas rendering. No error handling exists — a failed fetch leaves a blank canvas. HTMX views have no global error handler.

## Goals
1. Every fetch in reader.js has try-catch with user-visible error feedback
2. HTMX errors show a toast notification
3. Retry button on reader errors

## Decisions

### D1: Error overlay on canvas
A positioned div overlay on top of the canvas (not drawn on canvas itself) with error message and retry button. Overlay is hidden by default, shown on error, hidden again on successful load.

### D2: Toast for HTMX errors
A fixed-position toast element in the base layout. `htmx:responseError` event handler populates it and sets a 5s auto-dismiss timer. Uses existing Tailwind classes for styling.

### D3: Retry mechanism
Reader retry button calls the same fetch function that failed. HTMX retries are implicit (user clicks the link/button again).

## Risks
- Error overlay z-index must be above canvas but below nav bars — use z-20
- Toast must not interfere with reader overlay — different z-index layer

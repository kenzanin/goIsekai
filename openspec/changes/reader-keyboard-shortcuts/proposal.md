## Why
Reader has no keyboard navigation. Arrow keys for prev/next page, space for next page, escape to go back to detail — standard manga reader UX that users expect.

## What Changes
- Arrow left/right for prev/next page
- Space bar for next page
- Escape to go back to detail page
- Visual hint showing available shortcuts

## Capabilities
### New Capabilities
- `reader-keyboard-shortcuts`: Keyboard navigation for the manga reader canvas

### Modified Capabilities
(none)

## Impact
- `cmd/goisekai/frontend/lib/reader.js` — keydown event listener + navigation dispatch
- `internal/templates/views/reader.jet` — optional shortcut hint overlay
- No backend changes

## Context
Reader.js handles page navigation via click zones (left/right/top/bottom/center). No keyboard support exists. The reader uses canvas rendering with fetch-swap SPA navigation for chapter changes.

## Goals
1. Standard keyboard shortcuts for page navigation
2. Escape to exit reader
3. Chapter boundary crossing via keyboard

## Decisions

### D1: keydown listener on document
Add a `document.addEventListener('keydown', ...)` in reader.js. Check `document.activeElement` to skip when input/textarea is focused.

### D2: Reuse existing navigation functions
Keyboard handlers call the same `prevPage()`, `nextPage()`, `switchChapter()` functions already used by click zones. No new navigation logic needed.

### D3: Space bar prevention
Prevent default on Space to avoid page scroll. Only prevent when reader is active (not when searching or in other views).

### D4: Shortcut hint
Add a small "?" icon or tooltip showing available shortcuts. Optional — can be a simple title attribute on the reader container.

## Risks
- Space bar conflict with browser scroll — mitigated by preventDefault only when reader is active
- Escape conflict with browser fullscreen — acceptable, reader is not fullscreen

# Proposal

## Why

The two hand-written frontend JS files (`cmd/goisekai/frontend/lib/alpine-components.js`, 1158 LOC, and `reader.js`, 1560 LOC) have no JSDoc annotations. Static tooling (editor intellisense, `tsc --checkJs`, IDE hover) cannot see parameter/return types, so refactors rely purely on reading the code. The vendored minified files (`alpine.min.js`, `mermaid.min.js`) are out of scope.

## What Changes

- Add JSDoc headers (`@param`, `@returns`, `@typedef` for the shared state/prefs objects) to every top-level function and significant closure in `alpine-components.js` and `reader.js`.
- No runtime behavior change: comments only, verified byte-identical module behavior via the existing `TestFrontendJSParses` and a `just br` rebuild so the `.br` siblings match.
- No new lint tooling in this change — annotations are written to be valid for a future `tsc --noEmit --checkJs` or ESLint `jsdoc` plugin pass, but wiring that tooling is a separate change if wanted.

## Capabilities

### New Capabilities

- `frontend-jsdoc`: documentation annotations contract for the frontend JS sources.

## Impact

- Files touched: `cmd/goisekai/frontend/lib/alpine-components.js`, `cmd/goisekai/frontend/lib/reader.js` and their regenerated `.br` siblings (via `just br`).
- Risk: none at runtime; only cost is review of ~300 added comment lines.

## Context

See proposal.md for motivation. The constraints that shape the approach:

- `renderPage` (`internal/httpserver/routes.go:55`) already answers `X-Partial: true` with the view body only. The response therefore carries no `<main id="content">`, no navigation bar, and no `<title>`.
- `window.submitForm` (`cmd/goisekai/frontend/lib/alpine-components.js:22`) already performs exactly this fetch for POST `/action/*`: swap `#content`, re-run `Alpine.initTree(main)`, `history.replaceState`.
- The navigation bar is rendered once by the layout from `partials/nav.lua`, using the token the server injects as `data.active`. `navLink` (`partials/nav.lua:9`) emits anchors with no attribute identifying which one a token belongs to.
- The reader renders under `blank.lua` (`internal/httpserver/reader.go:85`) and owns its own `pushState`/`popstate` (`cmd/goisekai/frontend/lib/reader.js:398,625`).
- Static assets are served by `brHandler` (`internal/httpserver/static_br.go`), which prefers a `<name>.br` sibling and sets `Cache-Control: no-cache`. `make br` brotli-compresses `cmd/goisekai/frontend/lib/*.js` and `*.css` at build time.
- The layouts load Alpine from `cdn.jsdelivr.net` (`base.lua:50`, `blank.lua:21`) and Space Grotesk from `fonts.googleapis.com` (`base.lua:19`).
- Go's `mime` package already maps `.woff2` to `font/woff2`, so no MIME registration is needed for a vendored font.

## Goals / Non-Goals

**Goals:**

- Navigate internal links and internal GET forms in place, without rebuilding the document.
- Keep history correct: navigation adds an entry, an action does not.
- Keep the highlighted navigation item matching the page now shown.
- Remove the third-party CDN dependency for the framework and the webfont.

**Non-Goals:**

- Per-page browser titles. Both layouts hardcode `<title>goIsekai</title>`, so an in-place navigation cannot leave a stale title; introducing per-view titles is a separate change.
- The reader's shell and its history handling, which stay as they are.
- Partial responses gaining `<head>`, the navigation bar, or a full document. The server keeps answering body-only.
- PWA concerns: manifest, service worker, installability, offline behaviour. Separate change.
- Prefetching, page transitions, and DOM morphing beyond a body swap.

## Decisions

### 1. Extend the existing fetch helper; add no router library

Alternatives considered: HTMX `hx-boost` — rejected, because the capability carries an explicit "Remove HTMX dependency" requirement and Alpine already provides the toast, confirm and view-mode stores the app depends on. Hotwire Turbo — rejected, because it swaps `<body>` from a full document, which would discard the body-only `X-Partial` contract the server already implements, and it adds a second client framework beside Alpine.

Chosen because the pattern is proven in this file for POST actions, so the delta is a small addition to a file that already exists.

### 2. `pushState` for navigation, `replaceState` stays for actions

An action re-renders the page it was submitted from, so it must not add a history entry — `submitForm`'s existing `replaceState` is correct and is left alone. Navigation is a new entry; using `replaceState` there would make back step through states of one page instead of returning to the previous page.

### 3. The server sends the active navigation token; links carry `data-nav`

The navigation bar is rendered once by the layout, so an in-place swap cannot update it. The server already computes the token (`renderPage` receives `active`), so passing it to the client costs one response header.

Rejected: deriving the token client-side from the URL path — duplicates a mapping the server owns and drifts as routes are added. Rejected: including the navigation bar in the partial response — would stop the partial being body-only, a contract other callers now depend on.

The anchor needs a stable hook, so `navLink` gains `data-nav="<token>"` and the client moves the highlight by class using the same token vocabulary.

### 4. Delegated listeners on `document`

The body is replaced on every navigation, so listeners bound to elements inside it would be discarded. One `click` listener covers links and one `submit` listener covers GET forms; both call the same `navigate()`.

### 5. Scroll position recorded in history state

`pushState({scrollY})` records the position before leaving, and `popstate` restores `state.scrollY`; a fresh navigation scrolls to the top. Without this, back lands wherever the browser guesses, which is visible breakage on a long grid.

### 6. Exclusions bail out to the browser

Same-origin only; skip `target`, `download`, modifier-key clicks, non-`http(s)` schemes, and `#` fragments; skip `/action/` (forms own it), `/image`, `/plugin-static`, and `/view/read/` (the reader). Bailing out needs no server-side special case and leaves those paths working exactly as they do today.

### 7. Vendor the framework into `lib/`, vendor the font as plain files

`make br` globs `lib/*.js` and `lib/*.css`, so a vendored `lib/alpine.min.js` receives its `.br` sibling with no Makefile change. woff2 is already compressed, so brotli would save little and would require extending the Makefile to a subdirectory; the font files are therefore served as-is. The `@font-face` rules go into the existing inline `<style>` in `base.lua`, which also removes two `preconnect` hints and one render-blocking stylesheet request.

Considered and rejected: dropping the webfont and relying on the `system-ui` fallback already present in both places the font is used. That is a product decision about how headings and the logo render, not a technical one, so the font is vendored instead.

## Risks / Trade-offs

- **Widgets re-initialized after a swap** → the swap routine already calls `Alpine.initTree(main)`; the navigation path reuses that same routine so there is one place responsible for re-initialization.
- **`Cache-Control: no-cache` makes every navigation revalidate the shell** → against a localhost server this is a 304, and it is what keeps an unversioned asset URL from serving stale code after a rebuild. Left as is.
- **A route that answers a partial request with a redirect** → `redirect: 'manual'` makes the response opaque and unreadable, which is why the detail route already special-cases `X-Partial` (`internal/httpserver/views_manga.go:117`). Any new redirecting route needs the same treatment.
- **A vendored framework drifts from the published one** → pin and record the version beside the file, and update it deliberately rather than letting a CDN float.
- **The navigation token and the view's own active value disagree** → both are supplied by the same `renderPage` argument, so there is one source of truth.
- **A navigation while an action is in flight** → the last response to write `#content` wins. No queue is added, matching how actions already behave.
- **`alpine-spa-frontend`'s main spec does not currently validate** → it uses `## ADDED Requirements` where `openspec validate` requires `## Requirements`, so it fails today, and the archive merge has no `## Requirements` section to merge into. The heading must be normalized before this change is archived. The merge is otherwise unaffected.

## Migration Plan

Single deploy. No schema, config, or API change, and the app is localhost-only with no client state beyond the `localStorage` view-mode key, so rollback is reverting the commit. The vendored font files are new binaries committed with the change.

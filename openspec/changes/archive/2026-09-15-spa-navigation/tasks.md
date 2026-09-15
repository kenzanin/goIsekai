## 1. Self-hosted assets

- [x] 1.1 Vendor Alpine.js 3.14.9 (the version the layouts currently pull from jsdelivr) into `cmd/goisekai/frontend/lib/alpine.min.js`, recording the pinned version in a comment or the filename; verify `make br` produces `cmd/goisekai/frontend/lib/alpine.min.js.br` alongside it
- [x] 1.2 Vendor the Space Grotesk webfont weights 400, 500, 600 and 700 as woff2 files under `cmd/goisekai/frontend/lib/fonts/`; verify all four files are present and that requesting one through `/static/lib/fonts/<file>.woff2` returns `Content-Type: font/woff2`
- [x] 1.3 Replace the jsdelivr script tag and the Google Fonts preconnect/stylesheet links in `internal/templates/layouts/base.lua` and `internal/templates/layouts/blank.lua` with local paths, and add the `@font-face` rules to the existing inline `<style>` in `base.lua`; verify no third-party origin remains in either layout
- [x] 1.4 Verify a page still becomes interactive with all third-party origins unreachable — the toast and confirm stores and the view-mode toggle still work, proving the framework no longer depends on the CDN

## 2. Server-provided navigation token

- [x] 2.1 Add `data-nav="<token>"` to the anchor that `navLink` emits in `internal/templates/partials/nav.lua`; verify the rendered navigation carries one `data-nav` per link, using the same token vocabulary the server already passes as `active`
- [x] 2.2 Send the active navigation token to the client on partial responses from `renderPage` in `internal/httpserver/routes.go`, so an in-place swap can move the highlight without a re-render; verify a request with `X-Partial: true` carries the header and the token matches the view that was rendered
- [x] 2.3 Add a test in `internal/httpserver/` asserting the partial response for each view carries the header with the expected token; verify with `CGO_ENABLED=0 go test ./internal/httpserver/ -count=1`

## 3. Client-side navigation

- [x] 3.1 Add a `navigate(url, {push})` helper to `cmd/goisekai/frontend/lib/alpine-components.js` that fetches with `X-Partial: true`, swaps the `#content` body, re-runs `Alpine.initTree`, applies the navigation token, and uses `pushState` for navigation while leaving the existing `replaceState` action path alone; verify clicking a navigation link swaps the body and adds a history entry without a second document request in the network panel
- [x] 3.2 Add the delegated `click` listener on `document` with the exclusion rules from design decision 6 (same-origin only; skip `target`, `download`, modifier keys, non-`http(s)` schemes, `#` fragments, `/action/`, `/image`, `/plugin-static`, `/view/read/`); verify internal links swap in place while each excluded case performs a normal full-page navigation
- [x] 3.3 Add the delegated `submit` listener that routes internal GET forms through `navigate()`; verify submitting the library filter and the search form swaps the results in place without reloading the document
- [x] 3.4 Add the `popstate` handler and record and restore `scrollY` in history state; verify back and forward move between pages in place and a page left while scrolled returns to its previous position
- [x] 3.5 Handle a failed navigation: on a non-2xx response or a network error, fall back to a standard navigation and show the existing error toast; verify by pointing a link at a route that returns an error

## 4. Full verification

- [x] 4.1 Run `CGO_ENABLED=0 go test ./internal/... ./pkg/... ./cmd/... -count=1` and `make check`; verify both exit 0 with no new lint findings
- [ ] 4.2 Walk every view (library, search, detail, plugins, settings, logs, history, updates) in the browser; verify each one navigates in place, the highlighted navigation item follows the page, and no console error appears
- [ ] 4.3 Verify the reader keeps standard navigation in both directions — entering it from a detail page and leaving it — and that its own history handling and progress saving still work
- [ ] 4.4 Rebuild with `make build` and confirm the built binary serves the page with no third-party asset request in the network panel

## 5. Ready to archive

- [x] 5.1 Normalize `## ADDED Requirements` to `## Requirements` in `openspec/specs/alpine-spa-frontend/spec.md`; verify `openspec validate alpine-spa-frontend --type spec` passes, so the archive merge has a section to merge into

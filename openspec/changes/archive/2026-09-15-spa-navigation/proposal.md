# SPA Navigation

## Why

The `alpine-spa-frontend` capability already requires a client-side router that navigates without a full page reload, but only POST form actions ever got one (`window.submitForm`). Every navigation link and both GET search forms still tear down and rebuild the document, so the interface blanks, Alpine re-initializes and scroll position is lost on every page change. On a localhost-only server that reload is pure overhead: the server already knows how to answer with just the page body, because `renderPage` returns the partial when `X-Partial: true`. The router was specified in the `jet-to-lua-alpine-spa` change and never built.

## Changes

- Implement the in-place navigation the capability already describes: capture internal link clicks, fetch the target with `X-Partial: true`, and swap the page body instead of reloading the document.
- Treat GET form submissions (library filter, search page) as navigations too, so a search no longer reloads while an identically-targeted link does not.
- Honour the `pushState`/`popstate` contract so back and forward move between pages in place, and restore each entry's scroll position.
- Update the document title on navigation, because a partial response carries no `<title>`.
- Keep the highlighted navigation item correct after navigating in place. It is rendered once by the layout, so without this it stays on the page just left.
- Serve Alpine.js and the Space Grotesk webfont from the host's own asset root instead of `cdn.jsdelivr.net` and `fonts.googleapis.com`, so a blocked, slow or offline network cannot leave the interface without its framework.
- Leave the reader untouched: it uses its own shell with its own history handling and keeps loading normally.

## Capabilities

### Modified Capabilities

- `alpine-spa-frontend`: the "SPA client-side navigation" requirement gains the behaviour it currently leaves unspecified — GET forms as navigations, title updates, scroll restoration, and keeping the active navigation item correct — and a new requirement makes the client-side framework and its webfont self-hosted rather than CDN-loaded.

## Impact

- `cmd/goisekai/frontend/lib/alpine-components.js` — gains the navigation helper, the delegated link and form handler, and the `popstate` handler beside the existing `submitForm`.
- `cmd/goisekai/frontend/lib/` — new vendored Alpine.js build and Space Grotesk webfont files, each with the `.br` sibling that `make br` produces.
- `internal/templates/layouts/base.lua` and `internal/templates/layouts/blank.lua` — the jsdelivr script tag and the Google Fonts links are replaced by local ones.
- `internal/templates/partials/nav.lua` — each link carries its highlight token so the client can move the highlight without a re-render.
- `internal/httpserver/routes.go` — page renders carry the active navigation token to the client on partial responses.
- No plugin, database, HTTP API or reader change.

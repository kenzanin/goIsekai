---
type: Concept
title: Home reload NS_BINDING_ABORTED (saved-filter redirect)
description: Why Firefox logs NS_BINDING_ABORTED on plugin logos/favicon when the library reloads — the client-side location.replace() that restores the saved filter aborts the in-flight first navigation. Benign; fix options recorded but not applied.
tags:
  - goisekai
  - frontend
  - library
  - finding
generated:
  by: process:reasonix
  at: "2026-10-18T07:00:00Z"
---

# Home reload `NS_BINDING_ABORTED` (saved-filter redirect)

Observed: reloading the home page logs `NS_BINDING_ABORTED` in Firefox against
every plugin logo and the favicon, yet sending the same URLs by hand returns
200. Evidence: `start.har` (2026-10-18).

## It is not an HTTP status

`NS_BINDING_ABORTED` is Firefox's internal `NS_ERROR_ABORTED` — "the request
was cancelled because another navigation took over". In a HAR it is recorded as
`"status": 0`, with `time: 0`, `timings: {}` and `bodySize: -1`, i.e. cancelled
before the server was ever touched. So a 200 from a manual request is expected,
not a contradiction: the server never failed.

In `start.har` exactly one entry is aborted (entry 0, `/static/favicon.svg`,
referer `http://localhost:8080/`); every `plugin-static/*/logo.png` is 304 and a
second favicon fetch (entry 34) is 200, this time with referer
`...?sort=recent_read`. The aborted request belongs to the navigation that was
discarded, not the page that ended up on screen. Logos can be aborted too when
timing is slower; in this capture only the favicon was.

## Root cause

`cmd/goisekai/frontend/lib/alpine-components.js` runs `syncLibraryView()`
synchronously at script parse time (line ~1010; the `<script>` in
`internal/templates/layouts/base.lua:77` carries no `defer`). When the URL is a
bare `/` (or `/view/library`) with no params and localStorage
`gsk:library-view` holds a saved filter, it does:

```js
window.location.replace(window.location.pathname + '?' + saved);
```

Sequence: the browser starts loading `/`, kicks off subresource requests
(favicon, then the 28 logo images), the script executes, sees the saved filter,
and `replace()`s the location. The first navigation is cancelled → all its
in-flight requests are aborted → `NS_BINDING_ABORTED`. The replacement page
`/?sort=recent_read` loads and issues its own favicon/logo requests (200/304).

The saved-filter restore exists so a bare `/view/library` returns to the last
sort/status/tag the user picked (see the `gsk:library-view` block in the same
file).

## Impact: none

The aborted requests belong to a page that is deliberately thrown away. The
final page loads its own favicon and logos, so nothing is actually missing —
the plugin grid renders every logo. It is noise in the Network tab only.

Verified live: set `localStorage['gsk:library-view'] = 'sort=recent_read'`,
open bare `/` → the tab jumps to `/?sort=recent_read`, matching the HAR.

## Fix options (NOT applied — user judged it harmless)

The user chose to record the finding and fix only if it ever disrupts something.
If that changes, two candidate fixes, with trade-offs:

1. **Server-side 302** — persist the saved filter in a cookie and have the root
   handler redirect with `http.Redirect` before rendering. One navigation, no
   abort. Cost: a new cookie plus root-handler change.
2. **Defer the JS redirect** until the current load finishes. No architecture
   change, but flashes the unfiltered grid before jumping.

Do not "fix" it by making the assets synchronous or by removing `no-cache`:
neither is related, and the assets already serve correctly.

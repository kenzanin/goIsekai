---
type: Concept
title: Home reload NS_BINDING_ABORTED (saved-filter redirect)
description: Why Firefox logs NS_BINDING_ABORTED on plugin logos/favicon when the library reloads — the client-side location.replace() that restores the saved filter aborts the in-flight first navigation. Benign.
tags: [goisekai, frontend, library, finding]
generated: { by: agent/mcp, at: "2026-10-26T15:39:02Z" }
status: stable
---

# Home reload NS_BINDING_ABORTED (saved-filter redirect)

Observed: reloading the home page logs NS_BINDING_ABORTED in Firefox against every plugin logo and the favicon, yet sending the same URLs by hand returns 200. Evidence: start.har (2026-10-18).

## It is not an HTTP status

NS_BINDING_ABORTED is Firefox's internal NS_ERROR_ABORTED — "the request was cancelled because another navigation took over". In a HAR it is recorded as status: 0, with time: 0, timings: {} and bodySize: -1, i.e. cancelled before the server was ever touched.

## Root cause

`alpine-components.js` runs syncLibraryView() synchronously at script parse time when the URL is bare `/` with no params and localStorage gsk:library-view holds a saved filter it does `window.location.replace(window.location.pathname + '?' + saved)`. The browser starts loading /, kicks off subresource requests (favicon, logos), the script executes and replace()s the location. The first navigation is cancelled → all its in-flight requests aborted. The replacement page loads its own favicon and logos.

## Impact: none

The aborted requests belong to a page that is deliberately thrown away. The final page loads its own favicon and logos. It is noise in the Network tab only.

## Fix options (NOT applied — user judged it harmless)

If it changes, two candidate fixes: (1) Server-side 302 — persist the saved filter in a cookie and redirect before rendering. (2) Defer the JS redirect until current load finishes — no architecture change but flashes the unfiltered grid. Do not fix by making assets synchronous or removing no-cache.

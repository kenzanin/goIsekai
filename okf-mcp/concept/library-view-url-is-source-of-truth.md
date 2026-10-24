---
type: Concept
title: Library view URL is the source of truth (no localStorage filter persistence)
description: The stuck-filter bug: gsk:library-view localStorage restore made All/New/In-progress chips bounce back to the last filter; the rule, the New status chip, and the jscpd-driven history/updates merge.
tags:
  - goisekai
  - web-ui
  - library
  - localStorage
  - alpine
  - lua-templates
  - jscpd
generated:
  by: process:reasonix
  at: "2026-10-24T00:00:00Z"
---

# Library view URL is the source of truth

## The bug

Symptom: pick a status filter (e.g. "In progress"), then click "All" — the
page briefly shows all titles, then bounces back to the old filter. Same for
"New" → "All". The HAR showed `/view/library?status=reading` followed by a
restore to the saved query.

Root cause: `syncLibraryView` in `cmd/goisekai/frontend/lib/alpine-components.js`
persisted the last query string to `localStorage['gsk:library-view']` on every
param-carrying load and **restored it on any bare `/view/library` load**. A
bare load is also exactly what the "All" chip and the reset button navigate
to, so the restore re-applied the filter the user just cleared. WebKit (the
Wails client) made the bounce more visible but the mechanism was engine
independent.

## The rule

**The URL is the single source of truth for library filters/sort
(`?status=&tag=&sort=&q=&pluginID=`). Nothing is persisted.** A bare
`/view/library` means "show everything", period. `syncLibraryView` now only
**wipes the legacy `gsk:library-view` key** on a bare load (so old
installations self-heal once) and writes nothing. `gsk:view-mode`
(grid/list) is the one remaining localStorage key — it is view *mode*, not
filter state, and is reset by the sidebar titles card /
`window.resetLibraryView()`.

Do not re-add filter persistence here: every "restore on bare URL" variant
(restore on fresh tab, `history.state` check, sessionStorage flag) fails in
at least one real environment, because "typed the URL" and "clicked All"
are indistinguishable on load.

## New status chip

The toolbar status chips are defined in `internal/templates/views/library.lua`
(`statusOpts`): `all / reading / unread / done / new`. `new` filters to
titles with the `[New]` badge — server-side `filterLibrary`
(`internal/httpserver/views_library.go`) matches `stats[key].HasNew`, and the
sidebar "updates" card links to `?status=new`. The server already supported
the status; only the chip was missing.

## jscpd-driven merge (history ↔ updates)

`jscpd internal pkg cmd examples tools --min-tokens 80` flagged a 42-line
clone between `internal/templates/views/history.lua` and
`views/updates.lua` (the row `<a>` + cover + sticky section header). Resolved
in commit `9f9ca0f`: both views normalize their entry and call
`partials/manga_list_row` — a single `return function(data)` module (house
convention: every partial returns one callable; `data.Which` = `row` |
`section` dispatches). After the merge, jscpd shows 0 remaining template
clone pairs. The other big jscpd hits are the `examples/plugins/wasm/*`
sources, which are **intentionally** near-identical samples — do not "fix"
those.

## Verification

- `go test ./internal/templates/` (includes `TestSourcesCachedForEveryTemplate`,
  which requires every cached template source to contain `return function`)
- `go test ./internal/httpserver/` (includes
  `TestFrontendTemplateHandlersAreResolvable` — inline template handler names
  must exist in the `alpine-components.js` bundle)
- Live browser check: `/view/library?status=new` (63 titles) → click "All" →
  stays at 531 titles, tag/pagination links carry no `status=`; clicking
  "All" again stays "All".
- Any `alpine-components.js` edit needs `just br` (brotli siblings) before
  the frontend tests pass.

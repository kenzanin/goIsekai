---
type: Concept
title: Reader loading cancel fix
description: Cancel button during reader loading — AbortController aborts in-flight fetches, nav bar visible during load, spinner flash prevention in reader.lua.
tags:
  - goisekai
  - reader
  - frontend
  - finding
generated:
  by: process:omp-assistant
  at: "2026-10-11T02:58:59Z"
---

# Reader loading cancel fix

Problem (source: EchoesVault page `reader-loading-cancel-fix.md`): while the
reader was loading (spinner visible) the user could not cancel back to the
manga detail page except via browser Back. Requirements: nav bar visible
during loading, Back/cancel control returns to detail view, cancel aborts
in-flight reader fetches.

## Solution

### `cmd/goisekai/frontend/lib/reader.js`

- `loadAbortController` variable manages fetch aborts.
- Initial bar visibility changed `false` → `true` (bars visible during loading).
- `abortLoad()` cancels in-flight fetches via AbortController.
- `loadChapter()` fetch wired to the AbortController signal.
- Catch block checks `AbortError` and silently ignores aborted fetches.
- Back-link click handler aborts fetch before navigating.

### `internal/templates/views/reader.lua`

- Spinner div got `style="display:none"` to prevent flash of content.

## UX during loading

1. Top nav bar immediately visible with `← Back` link.
2. Clicking Back aborts in-flight reader fetches and navigates to the manga
   detail page.
3. Spinner behavior unchanged.

## Key route

Back link navigates to `/view/manga/{pluginID}/{mangaID}`, matching the route
defined in `internal/httpserver/routes.go:44`.

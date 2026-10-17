---
type: Concept
title: Reader two-page view (side-by-side and stacked)
description: The reader's spread layouts for sites that split one original page into two images, the counter/step contract, and the canvas render geometry for each.
tags:
  - goisekai
  - reader
  - frontend
  - feature
generated:
  by: process:reasonix
  at: "2026-10-17T07:35:00Z"
---

# Reader two-page view

Some sources serve one original page as two images (a half each). On a reader
set to one page per view that means two clicks to see one original page, and
continuous (strip) mode is not wanted for that — strip exists for manhwa.

## The layout is a tri-state, not a boolean

`spread` in `cmd/goisekai/frontend/lib/reader.js`:

| value   | on-screen layout                        |
| ------- | --------------------------------------- |
| `''`    | one page (default)                      |
| `side`  | two pages left/right — a page split horizontally |
| `stack` | two pages one above the other — a page split vertically, reads as one tall continuous page |

One button (`#btn-readmode`) cycles every read mode, showing the current letter
— A/B/C/D rather than icons, so the mode is readable at a glance:

| button | mode | state |
| ------ | ---- | ----- |
| `A` | 1 page per view (default) | `spread = ''`, `stripMode = false` |
| `B` | 2 pages per view, stacked | `spread = 'stack'` |
| `C` | 2 pages per view, side by side | `spread = 'side'` |
| `D` | continuous (vertical strip) | `stripMode = true` |

`currentReadMode()` derives the letter from state and `applyReadMode(mode)` sets
it; the two are the single source of truth for the button label and the cycle
order (`READ_MODES`). Strip is exclusive with the spread layouts — leaving `D`
goes through `exitStrip()`, and picking a non-D mode while in strip exits it —
so the whole thing is one mutually-exclusive choice seen from one control.

`side` in `rtl` direction reverses draw order so the earlier page sits on the
right; `stack` keeps reading order top to bottom.

**Legacy value:** earlier builds stored a boolean (`'1'`/`true`) for the
side-by-side view. It is still normalised to `'side'` on read, so old
localStorage and stored per-manga prefs keep working.

## Contracts that must hold

- **One step is one view.** `stepSize()` returns 2 when `spread` is set, 1
  otherwise; `next()`/`prev()` move by `stepSize()`, clamped to the page list.
  Hardcoding `current + 1` reintroduces the two-clicks-per-page bug.
- **A trailing odd page stands alone** rather than pairing with the next
  chapter's first page.
- **Strip mode wins.** `drawPage` only loads a second page when
  `spread && !stripMode`; turning a spread on while in strip calls `exitStrip()`.
- **The counter names the range**: `2-3 / 22` in a spread, `2 / 22` otherwise.
- **The pair scales as one unit.** `viewBox()` is the bounding box of what is on
  screen — `side` adds widths and takes the max height, `stack` adds heights and
  takes the max width. `render`, `calcBaseScale` and `clampPan` all read it, so
  both halves share one `baseScale` and zoom stays in step.

## Canvas geometry (verified from the browser console)

Measured on a real chapter, one 958x1120 and one 1125x1600 page:

- `side`  → `render: 2083x1600` (958+1125 wide, max(1120,1600) tall)
- `stack` → `render: 1125x2720` (max width, 1600+1120 tall)

This is what distinguishes the two layouts: `side` grows the width, `stack`
grows the height.

## State that must be reset

`img2` (the second page's `Image`) joins `img` in the resets in `drawPage` and
`commitChapter` — a stale `img2` would paint the old chapter's page.

## Persistence

Saved in the per-manga reader prefs blob (`POST /action/reader-prefs/...`) as
`spread`, alongside `viewMode`, `direction`, `strip`, `zoom`. The server treats
the blob as opaque JSON, so no Go-side change is needed to add a key.

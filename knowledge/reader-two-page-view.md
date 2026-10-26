---
type: Concept
title: Reader two-page view (side-by-side and stacked)
description: "The reader's spread layouts for sites that split one original page into two images, the counter/step contract, and the canvas render geometry for each."
tags: [goisekai, reader, frontend, feature, Concept]
generated: { by: agent/mcp, at: "2026-10-26T15:47:01Z" }
status: stable
---

## Status

- Fixed: roliascan and madarascans now ship a static-musl .so with zero DT_NEEDED entries (tier 1 in build-native.sh).
- NOT done: the reader two-page-view feature has no migration story yet (separate concept). This file only documents the reader layout model.

## The layout is a tri-state, not a boolean

spread in reader.js: '' = one page; side = two pages left/right (page split horizontally); stack = two pages stacked (page split vertically). Button A/B/C/D cycles every read mode.

| button | mode | state |
|---|---|---|
| A | 1 page per view (default) | spread='', stripMode=false |
| B | 2 pages, stacked | spread='stack' |
| C | 2 pages, side by side | spread='side' |
| D | continuous (vertical strip) | stripMode=true |

side in rtl reverses draw order. strip is exclusive with spread layouts.

Legacy boolean ('1'/true for side-by-side) normalised to 'side' on read.

## Contracts

- One step = one view: stepSize returns 2 when spread set, 1 otherwise; step moves by stepSize clamped to page list.
- Trailing odd page stands alone (not paired with next chapter's first page).
- Strip mode wins: drawPage loads second page only when spread && !stripMode.
- Counter names the range: 2-3 / 22 in spread, 2 / 22 otherwise.
- Pair scales as one unit: viewBox is bounding box of whats on screen.

## Canvas geometry

side - render: 2083x1600 (widths add, max height). stack - render: 1125x2720 (max width, heights add).

## State reset

img2 (second page Image) joins img in the resets in drawPage and commitChapter.

## Persistence

Saved in per-manga reader prefs blob (POST /action/reader-prefs) as spread, alongside viewMode, direction, strip, zoom. Server treats blob as opaque JSON.

## Migration

No special work needed: spread is just a new key in the opaque prefs JSON. Old readers that don't know it default to one-page view, which is the existing default.

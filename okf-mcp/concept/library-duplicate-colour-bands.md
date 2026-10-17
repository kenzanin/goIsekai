---
type: Concept
title: Library duplicate-group colour bands
description: How the library grid flags duplicate groups with a per-group left colour band, why the palette is indexed by group position, and the determinism it depends on.
tags:
  - goisekai
  - library
  - duplicates
  - frontend
  - feature
generated:
  by: process:reasonix
  at: "2026-10-16T00:00:00Z"
---

# Library duplicate-group colour bands

The library can group manga that look like the same story
(`DB.FindPotentialDuplicates`). The grid marks each group with a **3px left
border** in a colour unique to that group, so the same story is recognisable
across the grid even when its entries are far apart.

## Where it lives

| piece | file |
| ----- | ---- |
| group detection | `internal/database/duplicates_find.go` |
| palette + `dupColors`/`dupRank` build | `internal/httpserver/views_library.go` (`dupPalette`, `viewLibrary`) |
| `duplicates` sort | `internal/httpserver/library_sort.go` (`sortLibrary`) |
| band markup | `internal/templates/partials/library_cards.lua` |
| sort option | `internal/templates/views/library.lua` |

`viewLibrary` turns the group list into two maps keyed by manga row-ID:

- `dupColors`: row-ID → hex colour, handed to the template as `DupColors`.
- `dupRank`: row-ID → group index, read only by the `duplicates` sort.

`duplicateCount` is derived from `len(dupColors)`, so it counts manga that are
actually banded.

## The band is an inline style, not a Tailwind class

The colour comes from the server palette. Tailwind only generates classes it can
see spelled out in the scanned source (`tailwind.config.js` scans
`./internal/templates/**/*.lua`), so building `"border-" .. color .. "-500"`
would render nothing. The band therefore uses
`style="border-left:3px solid #rrggbb"`.

`border-left` (not `ring`, not a full `border`) was chosen deliberately: the card
is `rounded-lg overflow-hidden`, so a full border would shrink the cover. A left
border only takes 3px from the left edge — no visible cover distortion.

## Palette rules

`dupPalette` cycles by group position. Entries must be bright enough to read
against the `bg-neutral-900` card on a dark page, and must avoid the theme's
indigo accent — a group banded in the accent colour would look like a selected
card. Blacks and greys are useless as bands on a near-black card.

## Determinism is load-bearing

Colour = `dupPalette[groupIndex % len(dupPalette)]`, so **group order must be
stable** or bands change colour between page loads. Two things enforce it:

1. `FindPotentialDuplicates` sorts groups by member count **then by `Key`**
   (it previously used a bare `sort.Slice` with no tiebreak — equal-size groups
   could reshuffle between restarts and recolour the whole grid).
2. `sortLibrary(..., "duplicates")` places members first and orders by `dupRank`,
   so a group's cards stay adjacent instead of splitting across pages.

## Contracts

- **Every member of a group shares one colour.** Do not colour per manga.
- **`sortLibrary` takes `dupRank` and may receive nil** (a search skips duplicate
  detection). Nil must leave order untouched, not panic.
- **The `duplicates` sort keeps non-members below all members** via a stable
  sort, preserving their incoming order.
- **Verify a palette change by restarting twice** and confirming the band colours
  are byte-identical; a reorder means the determinism above broke.

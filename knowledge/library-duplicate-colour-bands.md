---
type: Concept
title: Library duplicate-group colour bands
description: "How the library grid flags duplicate groups with a per-group left colour band, why the palette is indexed by group position, and the determinism it depends on."
tags: [goisekai, library, duplicates, frontend, feature, Concept]
generated: { by: agent/mcp, at: "2026-10-26T15:42:30Z" }
status: stable
---

# Library duplicate-group colour bands

The library can group manga that look like the same story (DB.FindPotentialDuplicates). The grid marks each group with a 3px left border in a colour unique to that group.

## Where it lives

| piece | file |
| ----- | ---- |
| group detection | internal/database/duplicates_find.go |
| palette + dupColors/dupRank build | internal/httpserver/views_library.go (dupPalette, viewLibrary) |
| duplicates sort | internal/httpserver/library_sort.go (sortLibrary) |
| band markup | internal/templates/partials/library_cards.lua |
| sort option | internal/templates/views/library.lua |

viewLibrary turns the group list into two maps keyed by manga row-ID: dupColors (row-ID → hex colour) and dupRank (row-ID → group index). duplicateCount = len(dupColors).

## The band is an inline style, not a Tailwind class

The colour comes from the server palette. Tailwind only generates classes it can see spelled out in the scanned source (tailwind.config.js scans ./internal/templates/**/*.lua), so building border-<color>-500 would render nothing. The band uses style="border-left:3px solid #rrggbb". A left border only takes 3px from the left edge — no visible cover distortion.

## Palette rules

dupPalette cycles by group position. Entries must be bright enough to read against bg-neutral-900 and must avoid the theme's indigo accent. Blacks and greys are useless as bands on a near-black card.

## Determinism is load-bearing

Colour = dupPalette[groupIndex % len(dupPalette)], so group order must be stable or bands change colour between page loads. FindPotentialDuplicates sorts groups by member count then by Key; sortLibrary(\"duplicates\") places members first and orders by dupRank.

## Contracts

- Every member of a group shares one colour.
- sortLibrary takes dupRank and may receive nil (search skips duplicate detection) — nil must leave order untouched, not panic.
- The duplicates sort keeps non-members below all members via a stable sort.
- Verify a palette change by restarting twice and confirming band colours are byte-identical.

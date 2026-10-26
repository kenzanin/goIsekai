---
type: reference
title: UI Polish Review
description: Consolidated actionable UI/UX polish follow-up list for the reader.
tags: [goisekai, reader, ui, polish, reference]
generated: { by: agent/mcp, at: "2026-10-26T20:15:02Z" }
status: stable
---

# UI Polish Review

Consolidated actionable follow-up list for the goIsekai reader UI/UX polish.

Scope: `internal/templates/views/*.jet`, `layouts/base.jet`, `partials/nav.jet`, `cmd/goisekai/frontend/lib/reader.js`, generated `frontend/lib/tailwind.css`.

Legend: P0 = broken/invisible, P1 = noticeable improvement, P2 = nice-to-have.

## P0 — Broken / Invisible

### G1 · Reader bottom progress bar too thin
- Problem: read-progress line is `h-1.5` (6px), barely visible when nav auto-hides.
- Where: `internal/templates/views/reader.jet:64` (`#progress-line`)
- Fix: increase to `h-2` (8px) + subtle glow/outline.

### G2 · Library + detail pagination do not render
- Problem: pagination data computed in handlers but templates had no controls.
- Where: `internal/httpserver/views.go` + `views/library.jet`, `views/detail.jet`
- Fix (done): `viewMangaDetail` now slices chapters (page size 50) and passes `ChCurrentPage`/`ChTotalPages`/`ChHasNext`/`ChHasPrev`; `library.jet` gained Prev/Next + page indicator; detail.jet pagination above and below the list. Verify library renders at >24 titles and detail at >50 chapters.

### G3 · Search results lack full-title hover
- Fix (done): added `title="{{r.Title}}"` to search.jet title element.

## P1 — Noticeable Improvements

### D1 · Chapter action buttons are raw emoji
- Fix: `size-7` ghost-icon buttons with `border` + `hover:bg-neutral-800`.
### D2 · Detail cover image not aspect-ratio locked
- Fix: `aspect-[2/3]`.
### R1 · Reader page slider thumb barely visible
- Fix: `h-2` + larger accent thumb.
### R2 · Reader top bar wraps on narrow screens
- Fix: prioritize page counter + title; move zoom controls to bottom bar at `sm`.
### H1 · History relative timestamps update only on load
- Fix: `setInterval` refresh every 60s.

## P2 — Nice-to-have

### S1 · Search pagination link styling
### D3 · Detail genre pills capped (~6 + +N more)
### D4 · Zero-value dates show "Jan 1, 0001"
- Where: `views/detail.jet` `formatDate`
- Fix: hide date when timestamp is zero.
### G4 · tailwind.config.js palette unused
### G5 · No prefers-reduced-motion handling
### R3 · No pinch-zoom on the canvas
### PL1 · Plugins file input styling
### LG1 · Log lines not color-coded (WARN amber, ERROR red, DEBUG dim)

## Recommended Next Batch
1. G1 (progress bar height) 2. G3 verify + D2 (detail cover ratio) 3. D1 (chapter button chrome) 4. R1 (slider thumb) + R2 (top bar responsive)

> Last updated: 2026-09-04.

---
type: reference
title: goIsekai UI Specification
description: "goIsekai UI specification v1.0 — screens, navigation, design system."
tags: [goisekai, ui, spec, qml, reference]
generated: { by: agent/mcp, at: "2026-10-26T20:17:45Z" }
status: stable
---

# goIsekai UI Specification v1.0

**Target:** Desktop manga reader (Go + Qt/QML via miqt). **Frontend:** Plain HTML/CSS/JS.

## Screen Inventory

1. Library — grid of cover cards, sorted/paginated.
2. Search — plugin picker + query, results grid.
3. Manga Detail — cover, metadata, description, chapter list.
4. Reader — full-bleed image viewer, prev/next, keyboard shortcuts, progress.
5. Plugins — list installed, install via file picker.

## Navigation (Hash Router)

`#/library`, `#/search?plugin=<id>&q=<q>&page=<n>`, `#/manga/<pluginID>/<mangaID>`, `#/read/<pluginID>/<mangaID>/<chapterID>?page=0`, `#/plugins`.

## Reader Controls

- Click zones: left third = prev, right third = next.
- Keyboard: ←/A = prev, →/D = next, Space = next, Home = first, End = last, Esc = back to detail, F = fullscreen.
- Progress bar at top of reader.
- Auto-hide top/bottom bars on inactivity.

## Visual Design System (Dark Theme)

| Token | Hex | Usage |
| bg-primary | #0f0f13 | Page bg |
| bg-secondary | #1a1a24 | Cards, nav |
| accent | #7c5cfc | Buttons, links |
| text-primary | #e5e5e5 | Main text |
| text-secondary | #9898b0 | Metadata |

Typography: Inter 400/500/600/700. Spacing: 4/8/16/24/32/48.

## States

- Loading: spinner + "Loading…".
- Empty: icon + message + action.
- Error: inline "Failed to load" with retry button.

## Component Styles

Buttons (.btn/.btn-primary/.btn-ghost), inputs (.input/.select), cards (.manga-card with hover lift), badges (.status-ongoing/.status-completed), spinner (animated border-rotate).

## Interaction Spec

Library: click card → open manga. Search: Enter → search, click result → manga. Detail: "Add to Library" toggle. Reader: Space/Arrows navigate, progress persists via POST /api/progress.

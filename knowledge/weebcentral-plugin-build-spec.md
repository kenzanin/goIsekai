---
type: Concept
title: weebcentral.com plugin build spec from weebcentral_com_complete.har
description: "weebcentral.com plugin build spec dissected from a 20MB HAR (468 entries, 14 HTML docs) - endpoint shapes and anti-bot notes."
tags: [goisekai, plugins, weebcentral, har, build-spec]
generated: { by: agent/mcp, at: "2026-10-26T15:56:48Z" }
status: stable
---

# weebcentral.com plugin build spec

Source: `weebcentral_com_complete.har` (20 MB, 468 entries, 14 HTML docs). Dissected by WeebcentralHarDissect lane.

## Site identity

- Server-rendered HTML + HTMX, no JSON API. Cloudflare-fronted, anonymous.
- Hosts: `weebcentral.com` (pages), `temp.compsci88.com` (covers), `official.lowee.us` (reader images).
- Identity: manga = ULID at `/series/{ULID}/{Slug}`; chapter = ULID at `/chapters/{ULID}`.
- Cover ULID == URL ULID == cover-CDN filename (cross-check).

## URL shapes

| Purpose | URL |
|---|---|
| Search | `GET /search/data?display_mode=Full+Display&text={q}&sort=Best+Match&order=Descending` |
| Detail | `GET /series/{ULID}/{Slug}` |
| Chapters | `GET /series/{ULID}/full-chapter-list` (HX fragment, newest-first) |
| Pages | `GET /chapters/{ULID}/images?is_prev=False&reading_style=long_strip&current_page=1` (HX fragment) |

HTMX headers: `HX-Request: true`, `Referer`. Reader images need `Referer: https://weebcentral.com/`.

## Anchors

- Search cards: `article.bg-base-300.flex.gap-4.p-4` (32/page). Link = first `a[href*="/series/"]`. Cover = `picture source[srcset]` -> `/cover/normal/{ULID}.webp`.
- Detail: title = first `h1`, cover = `og:image`, status via `strong Status:`, genres via `a[href*="included_tag="]`.
- Chapters: rows `a[href^="/chapters/"]`, date = `time[datetime]` ISO.
- Pages: `#chapter-images > img` -> `@src`, `alt="Page {n}"`. 20 pages per chapter.

## Gaps

- `/search/data` and `/images` only observed WITH `HX-Request`.
- Always use `long_strip` reading style.

---
id: weebcentral-plugin-build-spec
title: "weebcentral.com — plugin build spec from weebcentral_com_complete.har"
description: "weebcentral.com plugin build spec dissected from a 20 MB HAR (468 entries, 14 HTML docs) — endpoint shapes and anti-bot notes for a future plugin."
type: Concept
tags: [goisekai, plugins, weebcentral, har, build-spec]
---

# weebcentral.com — plugin build spec from weebcentral_com_complete.har

Source: `weebcentral_com_complete.har` (20 MB, 468 entries, 14 HTML docs, 0 non-GET, 0 cookies).
Dissected by WeebcentralHarDissect lane; full report at `agent://WeebcentralHarDissect/report`.

## Site identity
- Server-rendered HTML + HTMX, no JSON API. Cloudflare-fronted, fully anonymous (no auth/cookies).
- Hosts: `weebcentral.com` (pages), `temp.compsci88.com` (covers), `official.lowee.us` (reader images).
- Identity: manga = ULID at `/series/{ULID}/{Slug}`; chapter = ULID at `/chapters/{ULID}`.
- Cover ULID == URL ULID == cover-CDN filename (free ID cross-check).

## URL shapes
| Purpose | URL |
|---|---|
| Search | `GET /search/data?display_mode=Full+Display&author=&text={q}&sort=Best+Match&order=Descending&official=Any&anime=Any&adult=Any` (+ `limit=32&offset=N` pages) |
| Detail | `GET /series/{ULID}/{Slug}` (full doc) |
| Chapters | `GET /series/{ULID}/full-chapter-list` (HX fragment, newest-first) |
| Pages | `GET /chapters/{ULID}/images?is_prev=False&reading_style=long_strip&current_page=1` (HX fragment) |

HTMX headers to reproduce: `HX-Request: true`, `HX-Target`, `HX-Trigger`, `Referer`.
Reader images need `Referer: https://weebcentral.com/`.

## Anchors
- Search cards: `article.bg-base-300.flex.gap-4.p-4` (32/page). Link = first `a[href*="/series/"]`
  (absolute `https://weebcentral.com/series/{ULID}/{Slug}`). Title = `a.line-clamp-1`
  (same href; mobile dup in `article.lg:hidden div.text-ellipsis.truncate`).
  Cover = `picture source[srcset]` → `/cover/normal/{ULID}.webp`
  (fallback `/cover/fallback/{ULID}.jpg`). Meta: `strong Status:`/`Type:`/`Year:` + span;
  authors `a[href*="search?author="]`; tags bare spans after `strong Tag(s):`.
  Pagination: trailing `button[hx-get*="offset="]` ("View More Results..."); absent = last page.
- Detail: title = first `h1` (desktop+mobile twins, same text). Cover = `og:image`
  (also `picture source[srcset]`). Author `<li>` with `strong` starting `Author(s):` →
  `a[href^="...search?author="]`. Status `<li>` `strong Status:` → link text
  (Ongoing/Complete/Hiatus/Canceled). Type same via `included_type=`. Genres `<li>`
  `strong Tags(s):` → all `a[href*="included_tag="]`. Description: `strong Description`
  next-sibling `p.whitespace-pre-wrap`. `#chapter-list` holds only 9 seed rows — always
  fetch full-chapter-list.
- Chapters: rows `a[href^="/chapters/"]`; id = ULID in href; name = first `span`
  inside `span.grow` (site-specific labels like `Night 121`, not `Ch. N`);
  date = `time[datetime]` ISO. Newest-first.
- Pages (two-step): `GET /chapters/{ULID}` doc has EMPTY `#chapter-images` shell
  (`max_page` embedded in Alpine init); then images endpoint returns
  `<section id="chapter-images">` with one `img` per page in order
  (`#chapter-images > img` → `@src`, `alt="Page {n}"`). `long_strip` returns ALL
  pages in one request (20/20 observed). Path `/manga/{Series-Slug}/{page}.png`.

## Gaps
- `/search/data` and `/images` only observed WITH `HX-Request` (likely fine without).
- Non-`long_strip` reading styles window per request — always use `long_strip`.
- Quick search `POST /search/simple` shape unknown — unused.

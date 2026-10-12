---
id: violetmanga-search-detail-findings
title: VioletManga search/detail selector findings
type: finding
tags: [violetmanga, plugin, selectors, lua]
---

# VioletManga findings

Base: `https://violetmanga.com`. WordPress bixbox/listupd theme. Static HTTP, no browser needed.

## Search
`GET /?s={query}` (`&paged=N` for page > 1).
- Cards: `.bsx > a` (href `/comics/{slug}/`) + `.tt` title + `a img[src]` cover.
- Store bare slug as id (`/comics/([^/]+)/?`), not full URL — full URLs 404 the `/view/manga` route (memory #4076).

## Detail
`GET /comics/{slug}/`
- Title: `h1.entry-title`
- Cover: `.thumb img[src]`
- Genres: `.genres-container .mgen a`
- Status: `.info-half .status i` via `host.text.normalize_status`
- Author: uploader only upstream, no mangaka — leave empty.

## Chapters
Same detail page SSR: `div.eplister#chapterlist`, free chapters are `a[href]` (`/{slug}-chapter-N/`); locked coin-gated anchors have no href.
- Store relative path (`violetmanga%.com(/[^%?#]*)`), URL-encoded at render time (#4076).

## Pages
`GET /{slug}-chapter-{N}/` — reader HTML is SSR, `ts_reader` images in static body. Pure-HTTP regex on `"images"` — no `host.browser.evaluate` needed.

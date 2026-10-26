---
description: "Base: https://zeroscann.com."
id: zeroscann-search-detail-findings
title: Zeroscann search/detail selector findings
type: finding
tags: [zeroscann, plugin, selectors, lua]
---

# Zeroscann findings

Base: `https://zeroscann.com`. Vuetify SPA-ish SSR. Static HTTP, images need Referer (403 without, 200 with).

## Search
`GET /manga?keyword={query}` (`&page=N` for page > 1).
- Cards are Vuetify `a.v-card--link`, NOT `.manga-card` (old plugin selector matched nothing).
- Scope per card around `div.zs-bg-3`: title anchors `a.zs-link`, covers `background-image` on `.v-image__image` inside cards.
- NEVER zip flat title/cover/link lists by index — the three lists differ in length. Join covers on thumb filename (≈ slug), with document-order fallback only when counts match.
- Store bare slug as id, not full URL (#4076).

## Detail
`GET /manga/{slug}`
- Title: `meta[property='og:title']`, cover: `meta[property='og:image']`, status: `meta[property='manga:status']`, genres: `a[href*='/manga/genre/']`.

## Chapters
Hrefs use `/chapter-N` (slash) NOT `-chapter-` (hyphen): match `/manga/[^/]+/chapter%-(.+)`. Decimal chapters exist (5.1/5.2) — pattern must be `[\d.]+` (#3871).

## Pages
Chapter HTML is plain SSR `<img src>` inside `.pages.longstrip` — pure HTTP, no CDP evaluate.

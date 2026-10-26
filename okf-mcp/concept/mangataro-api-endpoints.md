---
description: "Base: https://mangataro.org."
id: mangataro-api-endpoints
title: MangaTaro.org API endpoints
type: finding
tags: [mangataro, plugin, api, lua]
---

# MangaTaro API endpoints

Base: `https://mangataro.org`. All are plain HTTP GET, no token, no browser.

## Chapter list (all chapters in one call)
`GET /auth/manga-chapters?manga_id={numeric}&_t={token}&_ts={unix}`
- Needs headers: Referer (detail URL), Origin `https://mangataro.org`, sec-fetch-* — else 403.
- Token: `md5(unix .. "mng_ch_" .. YYYYMMDDHH_UTC)[:16]`. Hour is UTC, NOT local.
- Response: `{"success":true,"chapters":[{id,chapter,title,url}]}`, up to 500, no pagination.
- numeric id = `data-manga-id` from the detail page SSR (`host.html.find_attr(doc,"[data-manga-id]","data-manga-id")`).

## Page list (full deterministic image list, no browser)
`GET /auth/chapter-content?chapter_id={numeric}`
- Returns `{"success":true,"images":["https://mangataro.yachts/storage/chapters/{hash}/001.webp",...]}` — the complete list. No scroll-harvest needed (the reader lazy-loads via JS; a browser scroll race returns partial counts).
- `chapter_id` = the plain numeric chapter id stored in the DB (e.g. "9355").

## Search
`GET /?s={query}` — cards are `a[href*='/manga/']` wrapping `img` (alt=title, src=cover).

## Referer note
`mangataro.yachts` CDN fetches fine with or without Referer (unlike comix which needs none, zeroscann which needs one).

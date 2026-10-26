---
type: finding
title: Comizy.io plugin build spec
description: "Source: comizy_io_lengkap.har (559 entries, read-only dissection)."
tags: [goisekai, comizy, plugin, api, lua, nextjs, finding]
generated: { by: agent/mcp, at: "2026-10-26T15:41:16Z" }
status: stable
---

# Comizy.io plugin build spec

Source: comizy_io_lengkap.har (559 entries, read-only dissection). Next.js pages-router SSR (__N_SSP:true, buildId _HrvUeHQGJzVTKw7e8Ag3) fronting JSON REST at api.comizy.io. Covers rx.comizy.io/covers/*.webp, pages x1..x10.cmzcdn.org/e/*.webp.

## Rules

- No Authorization/Cookie on any read path. Only Origin/Referer: https://comizy.io + x-nextjs-data: 1 for /_next/data fetches.
- buildId from GET comizy.io/api/version at runtime — never hardcode.
- Images (covers + pages) need Referer: https://comizy.io/ — all observed fetch carried it, all 200.
- Plugin id = manga slug (detail/pages URLs key on slug). Chapters API needs the api id + cv — resolve via one detail fetch.
- Chapter slugs verbatim, never constructed.
- status casing differs: search "completed" vs detail "Ongoing" — normalize host-side.

## Endpoints

| ABI | URL |
|---|---|
| search | GET api.comizy.io/titles/search?q={q}&page={n}&limit=24 → data.items[]:{id,url,name,slug,cover,status,genres[],alt_names[],latest_chapters[]}, data.pagination |
| detail | GET comizy.io/_next/data/{buildId}/{slug}.json?slug={slug} → pageProps.initialManga:{id,name,cover,status,authors[],genres[],summary,firstChapter,cv} |
| chapters | GET api.comizy.io/titles/{mangaId}/chapters?cv={cv} → data.chapters[]:{id,url,name,slug,number} desc; number = global position, label from name |
| pages | GET comizy.io/_next/data/{buildId}/{slug}/{ch}.json?slug={slug}&chapter-slug={ch} → pageProps.initialChapter.images[] (+ pages[{url,width,height}]) |

## Gotchas

1. Detail-embedded initialManga.chapters[] capped at 50 — always call the chapters API.
2. cv is a router cache-buster, not a hard requirement.
3. Empty-search body shape never captured — guard on len(items)==0, not HTTP failure.
4. POST api.comizy.io/views is analytics — omit. api.comizy.io/genres is the genre-slug dictionary for ?genres= filter.

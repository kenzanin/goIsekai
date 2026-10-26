---
type: finding
title: MangaBuddy returned zero pages for every chapter - stale CDN host + cached empty answer
description: "MangaBuddy returned zero pages for every chapter - the site moved its CDN and the plugin had pinned the old host, while the host cached the empty answer."
tags: [goisekai, plugins, mangabuddy, lua, cache, chapter-pages, reader, finding]
generated: { by: agent/mcp, at: "2026-10-26T15:39:04Z" }
status: stable
---

# MangaBuddy returned zero pages for every chapter

Two defects: (1) the plugin pinned the old CDN host inside its page matcher (`cdn1.love4awalk.xyz`); the site moved to `cdn4.sags2mobile.com`, so the matcher matched nothing and every chapter came back with zero pages. (2) The host cached that empty answer and served it back, so the chapter could not recover even after the plugin was fixed.

## Defect 1: stale CDN host in matcher

`examples/plugins/lua/mangabuddy/main.lua` matched page images with `data-src="(https://cdn1\.love4awalk\.xyz/[^\"]*?/[\d.]+/\d+\.webp)"`. The site now serves from `cdn4.sags2mobile.com` — pattern matched 0 times. The matcher now keys on the page shape `data-src="(https://[^"/]+/[^"]*?/[\d.]+/\d+\.webp)"` ({host}/{slug}/{chapter}/{index}.webp). Cover and ad URLs don't have that shape, still skipped. Dead CDN constant removed.

## Defect 2: host cached empty answer

`internal/bridge/library_pages.go` stored whatever the plugin returned with no emptiness check; `GetPageListCached` returned a cached empty list as valid. Reader falls back to live fetch only on nil result, so one failed fetch pinned the chapter to "no pages for good". Fixed: `GetPageList` stores only non-empty; `GetPageListCached` treats cached empty as a miss.

## Self-healing verified live

Two chapters had empty `[]` cache. After fix, opening through reader endpoint refetched:

| chapter | cache before | cache after |
|---|---|---|
| uglymug-epicfighter.ZDOW2Q:chapter-1 | 2 B (`[]`) | 3,981 B (50 pages) |
| datsu-sara-nouka-no-isekai-slow-life.ZDOK7A:chapter-22 | 2 B (`[]`) | 2,823 B |

Empty cached page lists for mangabuddy: 0.

## Tests

- `internal/pluginmanager/mangabuddy_pages_test.go` — drives shipped Lua plugin against stub with current markup
- `internal/bridge/library_pages_test.go` — host rules: cached empty = miss, good cached = served, empty doesn't overwrite good

## Live evidence

- GET /api/sandbox/plugins/mangabuddy/pages/uglymug-epicfighter.ZDOW2Q:chapter-1 -> 200, 50 pages (was `[]`)
- GET /api/reader-data/mangabuddy/.../uglymug-epicfighter.ZDOW2Q:chapter-1 -> 200 with full page list
- GET /view/read/mangabuddy/...:chapter-1?page=1 -> 200, 160 chapters loaded
- GET /image?pluginID=mangabuddy&url=... -> 200, image/avif, 148,400 B

## Files

- `examples/plugins/lua/mangabuddy/main.lua` — host-agnostic page matcher, page index, dead CDN constant removed
- `internal/bridge/library_pages.go` — empty answers not cached; cached empty = miss
- `internal/pluginmanager/testdata/luaemptypages/main.lua` — fixture completed with full ABI

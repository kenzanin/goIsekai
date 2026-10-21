---
id: mangabuddy-empty-chapter-pages
title: MangaBuddy returned zero pages for every chapter - the site moved its CDN and the plugin had pinned the old host, while the host then cached the empty answer
type: finding
tags: [goisekai, plugins, mangabuddy, lua, cache, chapter-pages, reader]
---

# MangaBuddy returned zero pages for every chapter

Reported: *"mangabuddy gagal mendapatkan manga chapter bro tolong di fix"* with
the reader URL
`/view/read/mangabuddy/uglymug-epicfighter.ZDOW2Q/uglymug-epicfighter.ZDOW2Q:chapter-1?page=1`.

## Short answer

**Two defects, not one.**

1. **The plugin** pinned the old CDN host inside its page matcher. The site
   moved from `cdn1.love4awalk.xyz` to `cdn4.sags2mobile.com`, so the matcher
   matched nothing and every chapter came back with **zero pages**.
2. **The host** cached that empty answer and then served it back as an answer,
   so the chapter could not recover even after the plugin was fixed.

The chapter the user reported now returns **50 pages**; other chapters return
10, 35 and 76.

## Defect 1: the plugin pinned the CDN host

`examples/plugins/lua/mangabuddy/main.lua` matched page images with

```lua
data-src="(https://cdn1\.love4awalk\.xyz/[^"]*?/[\d.]+/\d+\.webp)"
```

The site now serves those images from `cdn4.sags2mobile.com`, so the pattern
matched **0** times - the plugin log read `pages: 0 for chapter ...`. The HTML
itself was fine (200, ~400 KB); only the matcher was stale. Search, detail and
the chapter list were all unaffected, which is why the plugin looked healthy
apart from the reader.

The matcher now keys on the page **shape** instead of the host:

```lua
data-src="(https://[^"/]+/[^"]*?/[\d.]+/\d+\.webp)"
```

The shape is `{host}/{slug}/{chapter}/{index}.webp`. The cover
(`/thumb/...`) and the ad script URLs do not have that shape, so they are
still skipped - verified against the live markup: 50/50 matches on chapter-1,
10/10 on chapter-70.8, and **0** `data-src` attributes left unmatched.

**Swapping the host for the new one would have broken again on the next CDN
move** - which is exactly what happened. The dead `CDN` constant was removed
so the stale host cannot be reused, and each page now carries an `index` like
the other Lua plugins do.

## Defect 2: the host cached the empty answer

`internal/bridge/library_pages.go` stored whatever the plugin returned:

```go
if raw, merr := json.Marshal(result); merr == nil {   // no emptiness check
    s.db.SaveChapterPages(chapterIntID, raw)
}
```

and `GetPageListCached` returned a cached empty list as a valid answer. The
reader only falls back to a live fetch on a **nil** result
(`internal/httpserver/reader_view.go:87`), so one failed fetch pinned the
chapter to "no pages" **for good** - the chapter could not recover even after
the plugin was fixed, and the stale `[]` was sitting in the database.

Both halves are fixed:

- `GetPageList` stores the page list only when it is **non-empty**;
- `GetPageListCached` treats a cached empty list as a **miss** (returns nil),
  so the next read refetches.

This mirrors the empty-guard `GetMangaDetail` and `GetChapterList` already had
in `internal/pluginmanager/api_detail.go` - the page list was the one ABI call
that never got one.

## Self-healing verified live

Two chapters had an empty `[]` in the cache. After the fix, opening them
through the reader's own endpoint refetched and replaced it:

| chapter | cache before | cache after |
|---|---|---|
| `uglymug-epicfighter.ZDOW2Q:chapter-1` | 2 B (`[]`) | 3,981 B (50 pages) |
| `datsu-sara-nouka-no-isekai-slow-life.ZDOK7A:chapter-22` | 2 B (`[]`) | 2,823 B |

Empty cached page lists for mangabuddy: **0**.

## Tests

- `internal/pluginmanager/mangabuddy_pages_test.go` - drives the shipped Lua
  plugin against a stub serving the site's **current** markup: the page list
  is found, the cover and ad script are not mistaken for pages, the pages are
  numbered in reading order, and a failing site yields zero pages.
- `internal/bridge/library_pages_test.go` - the host rules: a cached empty list
  reads as a miss, a good cached list is still served, an empty answer does
  not overwrite a good cache.

The `luaemptypages` fixture had to be completed with `get_manga_detail` and
`get_chapter_list`: with only `search_manga` and `get_page_list` it failed
the load-time ABI verification, so the plugin could never be active and the
live path could not be exercised.

## Live evidence

- `GET /api/sandbox/plugins/mangabuddy/pages/uglymug-epicfighter.ZDOW2Q:chapter-1`
  -> 200, 50 pages (was `[]`)
- `GET /api/reader-data/mangabuddy/uglymug-epicfighter.ZDOW2Q/uglymug-epicfighter.ZDOW2Q:chapter-1`
  -> 200 with the full page list (the endpoint the reader itself uses)
- `GET /view/read/mangabuddy/uglymug-epicfighter.ZDOW2Q/uglymug-epicfighter.ZDOW2Q:chapter-1?page=1`
  -> 200, 160 chapters loaded
- `GET /image?pluginID=mangabuddy&url=...` -> 200, image/avif, 148,400 B

## Files

- `examples/plugins/lua/mangabuddy/main.lua` - host-agnostic page matcher,
  page `index`, dead `CDN` constant removed
- `internal/bridge/library_pages.go` - empty answers are not cached; a cached
  empty list is a miss
- `internal/pluginmanager/testdata/luaemptypages/main.lua` - fixture completed
  with the full source-plugin ABI

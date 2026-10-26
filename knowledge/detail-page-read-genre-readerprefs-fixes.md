---
type: finding
title: "Detail-page read toggle, genre tags, and reader-prefs persistence"
description: Three detail-page/reader bugs and their root causes
tags: [goisekai, reader, detail-page, enrichment, genres, sqlite, bugfix, finding]
generated: { by: agent/mcp, at: "2026-10-26T15:40:04Z" }
status: stable
---

# Three detail-page/reader bugs and their root causes

All three looked like frontend bugs; two were server-side, one was JS. Verified end-to-end against a running server (curl + browser), not by reading only.

## 1. Chapter read toggle could not be un-set

The chapter row renders the derived Done flag, not is_read:

- chapter_progress.Done = IsRead || (TotalPages > 0 && LastPageRead >= TotalPages)
- detail_chapters.lua uses p.Done for the strikethrough + green tick.

ToggleChapterRead flipped only is_read, so a fully-read chapter (last_page_read >= total_pages, is_read = 0) stayed Done and a second click could not reset it.

**Fix** (internal/database/chapters_query.go): the toggle now flips the whole derived state in one statement — is_read becomes 0 when the row was done-or-read, and last_page_read is cleared only when the row was fully read. A partially-read chapter keeps its bookmark. Regression test: TestToggleChapterReadResetsFullyReadChapter. ResetChapterProgress already cleared both signals and is unchanged.

## 2. Genre tags absent and the enrichment panel could not modify them

cachedMangaFallback (internal/bridge/library_fallback.go) built the manga from the DB row but set neither RawGenres nor — without a user override — Genres. The detail page takes this cached path whenever the manga has persisted chapters, so genres were empty and the enrichment panel could not mark chips active.

Fix: the cached path now mirrors enrichment categories into RawGenres, and falls back to them for Genres when no override exists. Regression test: TestCachedMangaFallbackPopulatesGenres.

## 3. Per-manga reader prefs reset on chapter switch / page turn

The server side was already correct. Two JS faults in cmd/goisekai/frontend/lib/reader.js:

- savePrefs() debounced 600 ms with no unload flush. Switching chapter is a full page load, so a change made less than 600 ms before navigating was thrown away.
- restoreZoom() read the server snapshot first, which is re-read on every image load, so it snapped zoom back on each page turn.

Fix: split postPrefs(keepalive), add flushPrefs() on pagehide + visibilitychange (POST with keepalive: true), and make restoreZoom prefer the localStorage mirror, written synchronously on every change. Regression test: TestReaderPrefsSurviveAChapterSwitch (reader.js is a self-executing IIFE with no exports).

## Reproduce / verify

```
sqlite3 app_data/goisekai.db "UPDATE chapters SET is_read=0,last_page_read=11,total_pages=11 WHERE source_chapter_id='<cid>';"
curl -b cj -X POST localhost:8080/action/toggle-read/<plugin>/<manga>/<cid> -H 'X-Partial: true' ...
sqlite3 app_data/goisekai.db "SELECT is_read,last_page_read FROM chapters WHERE source_chapter_id='<cid>';"  # expect 0|0
curl -s localhost:8080/view/manga/<plugin>/<manga> | grep -c genre-tag  # expect > 0
```

After editing anything under cmd/goisekai/frontend/lib/, run just br — the .br sibling is what the server sends (TestBrotliSiblingsMatchSource).

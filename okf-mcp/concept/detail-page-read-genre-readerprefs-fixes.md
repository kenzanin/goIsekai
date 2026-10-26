---
description: "Three detail-page/reader bugs and their root causes"
type: finding
title: Detail-page read toggle, genre tags, and reader-prefs persistence
tags: [bugfix, reader, detail-page, enrichment, genres, sqlite]
---

# Three detail-page/reader bugs and their root causes

All three looked like frontend bugs; two were server-side, one was JS. Verified
end-to-end against a running server (curl + browser), not by reading only.

## 1. Chapter read toggle could not be un-set

The chapter row renders the **derived** `Done` flag, not `is_read`:

- `chapter_progress.Done = IsRead || (TotalPages > 0 && LastPageRead >= TotalPages)`
  (`internal/database/chapters_read.go`, `GetChapterProgressForManga`).
- `detail_chapters.lua` uses `p.Done` for the strikethrough + green tick.

`ToggleChapterRead` flipped **only** `is_read`, so a fully-read chapter
(`last_page_read >= total_pages`, `is_read = 0`) stayed `Done` and a second click
could not reset it.

**Fix** (`internal/database/chapters_query.go`): the toggle now flips the whole
derived state in one statement — `is_read` becomes 0 when the row was
done-or-read, and `last_page_read` is cleared **only** when the row was fully
read. A partially-read chapter keeps its bookmark, because only `is_read` can
make it Done. Regression test: `TestToggleChapterReadResetsFullyReadChapter`.
`ResetChapterProgress` already cleared both signals and is unchanged.

## 2. Genre tags absent and the enrichment panel could not modify them

`cachedMangaFallback` (`internal/bridge/library_fallback.go`) built the manga
from the DB row but set **neither** `RawGenres` nor — without a user override —
`Genres`. The detail page takes this cached path whenever the manga has
persisted chapters, so:

- `views/detail.lua` rendered no genre tags (`genres = manga.Genres or {}` was empty),
- `detail_alt.lua` got `PluginGenres = manga.RawGenres` = nil, so **no** chip was
  marked active and every chip rendered as `add-category`; clicking one toggled
  genre off-then-on, i.e. nothing appeared to happen.

Plugin genres have no per-manga column: the **enrichment categories** are their
persisted mirror (`manga_categories`, the same source `displayedGenres` falls
back to). **Fix**: the cached path now mirrors those categories into
`RawGenres`, and falls back to them for `Genres` when no override exists.
Regression test: `TestCachedMangaFallbackPopulatesGenres`.

## 3. Per-manga reader prefs reset on chapter switch / page turn

The server side was already correct (`reader_prefs` column, `POST
/action/reader-prefs` → 204, `data-manga-prefs` embedded by `viewReader`). Two
JS faults in `cmd/goisekai/frontend/lib/reader.js`:

- `savePrefs()` debounced 600 ms with **no unload flush**. Switching chapter is a
  full page load, so a change made less than 600 ms before navigating was thrown
  away and the next chapter read the old blob.
- `restoreZoom()` read the **server** snapshot first. That snapshot is only as
  fresh as the last settled POST, and it is re-read on every image load, so it
  snapped the zoom back on each page turn.

**Fix**: split `postPrefs(keepalive)`, add `flushPrefs()` on `pagehide` +
`visibilitychange` (POST with `keepalive: true`), and make `restoreZoom` prefer
the localStorage mirror, which is written synchronously on every change.
Regression test: `TestReaderPrefsSurviveAChapterSwitch` (shape pin — reader.js is
a self-executing IIFE with no exports).

## Reproduce / verify

```bash
# 1: fully-read row must lose Done on one click
sqlite3 app_data/goisekai.db "UPDATE chapters SET is_read=0,last_page_read=11,total_pages=11
  WHERE source_chapter_id='<cid>';"
curl -b cj -X POST localhost:8080/action/toggle-read/<plugin>/<manga>/<cid> -H 'X-Partial: true' ...
sqlite3 app_data/goisekai.db "SELECT is_read,last_page_read FROM chapters WHERE source_chapter_id='<cid>';"
# expect 0|0  (Done cleared)

# 2: detail page must render genre chips
curl -s localhost:8080/view/manga/<plugin>/<manga> | grep -c genre-tag   # expect > 0

# 3: prefs must survive an immediate chapter switch (inside the 600ms debounce)
# click fit mode, navigate at once, then read mangas.reader_prefs
```

Gotcha: after editing anything under `cmd/goisekai/frontend/lib/`, run `just br`
— the `.br` sibling is what the server actually sends, and a stale one ships the
old JS silently (`TestBrotliSiblingsMatchSource`).

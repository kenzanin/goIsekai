---
id: chapter-bound-zero-chapter-num
title: Bulk "set this and before/after as read" hit the whole manga when chapter_num is 0
type: finding
tags: [goisekai, chapters, reading-progress, sqlite, comizy, bug]
---

# Bulk "set this and before/after as read" hit the whole manga when chapter_num is 0

Reported 2026-10-20 from a detail-page action (`manga_detail_action.har`): tick
chapter 5 → "Set up to as read" marked **every** chapter read, and "Set down to
as unread" likewise flipped the whole list. The HAR confirmed the client was
innocent — it posted exactly one `chapterIDs` value:

```
POST /action/chapter-actions
action=set-up-to-read
chapterIDs=isekai-...-o-tsukuru|chapter-5      ← one chapter
```

## Root cause

`setChaptersToBound` (`internal/database/chapters_read.go`) derived a scalar
`bound` from the ticked rows' `chapter_num` and then filtered purely on that
column:

```go
range_ := Chapters.ChapterNum.LT_EQ(Float(bound))   // "up to"
range_ = Chapters.ChapterNum.GT_EQ(Float(bound))    // "down to"
```

The `comizy` source emits **no chapter numbers**, so every row of that manga has
`chapter_num = 0.0` (deliberate: the Lua and Odin plugins both leave it at 0 —
see the note at the top of `comizy_odin/main.odin`, which explicitly warns that
inventing numbers would detach the reader from recorded progress). Bound 0 makes
`chapter_num <= 0` true for **all** rows, and `>= 0` true for all rows too. The
same shape affects **12 mangas** in the reference DB.

Proof from the DB before the fix:

```
39778|chapter-1|0.0   39782|chapter-5|0.0
39779|chapter-2|0.0   39783|chapter-6|0.0
39780|chapter-3|0.0   39784|chapter-7|0.0
39781|chapter-4|0.0
```

## Fix

`chapter_num` alone is not a key. The bound is now the pair
`(chapter_num, id)` and the predicate is lexicographic — `id` is insertion
order, which is the source's own chapter order, so it is the correct tie-break
and matches what the detail page shows (`ORDER BY chapter_num DESC, id DESC`):

```go
eqNum  := Chapters.ChapterNum.EQ(Float(boundNum))
range_ := Chapters.ChapterNum.LT(Float(boundNum)).
    OR(eqNum.AND(Chapters.ID.LT_EQ(Int(boundID))))
```

`id` also breaks the tie when a source ships *partial* numbers with duplicates,
not only the all-zero case.

## Why the existing test missed it

`TestSetChaptersBulkRead` builds four chapters numbered 1..4, so the bound was
always unique and `<=`/`>=` on the number happened to select the right range.
`TestSetChaptersBoundWithZeroChapterNums` adds seven chapters at `chapter_num 0`
and asserts both directions from the middle one:

| action (ticked ch5) | expected |
|---|---|
| `set-up-to-read` | ch1–5 read, ch6–7 unread |
| `set-down-to-unread` | ch1–4 read, ch5–7 unread |

## Verified live

Against the real `comizy` manga from the HAR (`plugin_cache` irrelevant here;
this is the `chapters` table): `set-up-to-read` on ch5 → ch1–5 `is_read=1`,
ch6–7 `0`; then `set-down-to-unread` on ch5 → ch1–4 `1`, ch5–7 `0`. The user's
rows were reset to all-unread afterwards.

## Related trap

`app_data` holds `goisekai-tray` on port 8080, not just `goisekai`. A restart
that only runs `pkill -x goisekai` leaves the tray process bound and the new
server dies with `bind: address already in use` — kill both.

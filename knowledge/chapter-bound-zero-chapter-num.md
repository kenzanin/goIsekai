---
type: finding
title: "Bulk \"set this and before/after as read\" hit the whole manga when chapter_num is 0"
description: "Bulk \"set this and before/after as read\" hit the whole manga when chapter_num is 0"
tags: [goisekai, chapters, reading-progress, sqlite, comizy, bug, finding]
generated: { by: agent/mcp, at: "2026-10-26T15:39:01Z" }
status: stable
---

## Root cause

`setChaptersToBound` (`internal/database/chapters_read.go`) derived a scalar `bound` from the ticked rows' `chapter_num` and then filtered purely on that column:

```go
range_ := Chapters.ChapterNum.LT_EQ(Float(bound))  // "up to"
range_ = Chapters.ChapterNum.GT_EQ(Float(bound))    // "down to"
```

The `comizy` source emits no chapter numbers, so every row of that manga has `chapter_num = 0.0`. Bound 0 makes `chapter_num <= 0` true for all rows, and `>= 0` true for all rows too.

Proof from the DB before the fix:

```
39778|chapter-1|0.0   39782|chapter-5|0.0
39779|chapter-2|0.0   39783|chapter-6|0.0
39780|chapter-3|0.0   39784|chapter-7|0.0
39781|chapter-4|0.0
```

## The fix

`chapter_num` alone is not a key. The bound is now the pair `(chapter_num, id)` and the predicate is lexicographic — `id` is insertion order, which is the source's own chapter order:

```go
eqNum  := Chapters.ChapterNum.EQ(Float(boundNum))
range_ := Chapters.ChapterNum.LT(Float(boundNum)).
    OR(eqNum.AND(Chapters.ID.LT_EQ(Int(boundID))))
```

`id` also breaks the tie when a source ships partial numbers with duplicates.

## Verification

| action (ticked ch5) | expected |
|---|---|
| `set-up-to-read` | ch1–5 read, ch6–7 unread |
| `set-down-to-unread` | ch1–4 read, ch5–7 unread |

Live: Against the real comizy manga from the HAR: set-up-to-read on ch5 → ch1–5 read, ch6–7 unread; then set-down-to-unread on ch5 → ch1–4 read, ch5–7 unread.

## Related

`app_data` holds `goisekai-tray` on port 8080, not just `goisekai`. A restart that only runs `pkill -x goisekai` leaves the tray process bound and the new server dies with bind conflict.

---
description: "Sync never pruned the chapter rows a plugin re-issued"
id: stale-chapter-rows-prune
title: Sync upserts chapters but never pruned the rows a plugin re-issued - 196 rows were provably superseded duplicates, now removed under a proof rule
type: finding
tags: [goisekai, database, chapters, sync, prune, plugins, lhtranslation, mangasushi, go-jet]
---

# Sync never pruned the chapter rows a plugin re-issued

Raised as two side findings after the `released_at` backfill: *"stale/orphan
chapter rows in the DB that sync never prunes"* and *"duplicate chapter rows"*.

## Short answer

**One real defect, now fixed.** `persistMangaDetails` only ever upserted, so
when a plugin re-issued a chapter under a new id the old row stayed forever.
**196** rows were provably superseded duplicates; the sync now removes them
under a rule that needs complete proof.

**The other claim was wrong.** The "8,397 duplicate rows" are the *site's* own
repeated chapter numbers, not a host bug.

## Two earlier numbers were wrong, and why

Both of my first measurements were artifacts, and the corrected numbers are
much smaller:

| first claim | what it actually was | real number |
|---|---|---|
| "10,790 orphan rows" | a `LIKE source_manga_id \|\| '%'` query - every plugin joins slug and chapter id with a **different** separator (`:`, `/`, `/c001`, UUID), so the prefix test matched nothing meaningful | **296** stale rows |
| "8,397 duplicate rows" | mangafire's own answer contains two chapters with the same `chapter_num` and different ids/dates - the site really publishes repeated numbers | not a host bug |

`mangafire/62q4` is the proof: `The Healers' Guild` appears twice, both with
`chapter_num = 1`, ids `7633819` and `4341176`. Two different chapters, one
number - the site's doing.

## The one real defect

`internal/bridge/sync.go` `persistMangaDetails` upserted every chapter the
plugin returned and stopped there. lhtranslation and mangasushi used to emit
bare ids (`chapter-53`) and now emit `<slug>:chapter-53`, so a re-sync creates
the new row and orphans the old one - forever, because nothing prunes.

## What is provable, and what is not

Measured over all 519 library manga, comparing the database against the
plugin's **live** answer for each one:

| class | rows | verdict |
|---|---|---|
| superseded (plugin now sends `<slug><sep><old-id>`) | **196** | prunable |
| no provable replacement (mangzio 1, mangahub 3, comix 4) | 8 | kept |
| plugin answered 0 chapters (mangadex fetch failures) | 92 | kept |

**The 92 are the important trap.** A plugin that failed to list its chapters
proves nothing about rows it never sent - treating an empty answer as "the
manga has no chapters" would have deleted 92 real rows.

## The rule (internal/database/chapters_prune.go)

A row is deleted only when **all** of these hold:

1. the plugin did **not** send that id in this answer;
2. the id's **tail** after a separator (`:#?=&%!@~,;|`) equals the tail of an id
   the plugin **did** send - the tail must be at least 3 characters, so a short
   id like `1` cannot match an unrelated `chapter-1`;
3. the chapter **number** is the same;
4. the tail is **unambiguous** - never seen with two different chapter numbers;
5. the row holds **no** read progress, skip or download.

An empty answer removes nothing. A row the plugin still sends is never a
candidate.

The rule is deliberately narrow: it assumes a plugin re-issues ids by
**prefixing** the old one. A plugin that renames ids arbitrarily leaves its old
rows behind - the `ponytail:` comment in the file names the upgrade path.

## Why not DeleteChaptersNotIn

`internal/database/chapters_query.go` already has `DeleteChaptersNotIn(mangaID,
keepIDs)`, and it is exactly the wrong tool: it removes **every** row not in the
keep list, which would have taken the 8 unprovable rows and the 92 failed-fetch
rows with them. The new function is a separate, targeted delete.

## Live result

The prune ran against the live database for the 5 affected manga and removed
**exactly** the predicted 196 rows (3 + 81 + 6 + 50 + 56), taking the chapter
table from **31,307 to 31,111**. After it, the database matches the plugin's
own answer exactly for all 5 (7/7, 50/50, 57/57, 82/82, 6/6), with 0 orphan
`read_history` rows, 0 orphan `chapter_pages` rows and 0 orphan chapters.

## Files

- `internal/database/chapters_prune.go` - `PruneSupersededChapters`, built on
  the go-jet DSL (`Chapters.SELECT` / `Chapters.DELETE` / `ID.IN`), no
  hand-written SQL
- `internal/database/chapters_prune_test.go` - four regression tests: a proven
  supersede is removed, unproven rows stay (read / number mismatch / ambiguous
  tail), an empty answer removes nothing, a live row is never touched
- `internal/bridge/sync.go` - calls the prune at the end of
  `persistMangaDetails` with the ids and numbers the plugin just returned

**Rule:** a chapter row is removed only when its replacement was in the same
answer. An empty answer is not evidence.

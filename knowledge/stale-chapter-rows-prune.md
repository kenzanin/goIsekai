---
type: finding
title: Sync upserts chapters but never pruned superseded rows - 196 removed
description: "Sync upserts chapters but never pruned the rows a plugin re-issued - 196 rows were provably superseded duplicates, now removed."
tags: [goisekai, database, chapters, sync, prune, plugins, lhtranslation, mangasushi, finding]
generated: { by: agent/mcp, at: "2026-10-26T15:39:06Z" }
status: stable
---

# Sync never pruned the chapter rows a plugin re-issued

One real defect: persistMangaDetails only ever upserted, so when a plugin re-issued a chapter under a new id the old row stayed forever. 196 rows were provably superseded duplicates; the sync now removes them under a proof rule. The "8,397 duplicate rows" claim was wrong — that was mangafire's own repeated chapter numbers.

Both earlier numbers were artifacts:
- "10,790 orphan rows" was a LIKE prefix query that matched nothing meaningful (plugins use different separators: :, /, /c001, UUID) → real: 296 stale rows
- "8,397 duplicate rows" are the site's own repeated chapter numbers → not a host bug

## The one real defect

internal/bridge/sync.go persistMangaDetails upserted every chapter the plugin returned and stopped. lhtranslation and mangasushi used to emit bare ids (chapter-53) and now emit <slug>:chapter-53, so a re-sync creates the new row and orphans the old one forever.

## The rule (internal/database/chapters_prune.go)

A row is deleted only when all of these hold:
1. plugin did NOT send that id in this answer;
2. the id's tail after a separator (:#?=&%!@~,;|) equals the tail of an id the plugin DID send — tail must be at least 3 characters;
3. the chapter number is the same;
4. the tail is unambiguous — never seen with two different chapter numbers;
5. the row holds no read progress, skip or download.

An empty answer removes nothing. A row the plugin still sends is never a candidate. The rule assumes prefixing; arbitrary renames leave old rows (upgrade path noted in ponytail: comment).

## Live result

The prune ran against the live database for the 5 affected manga and removed exactly the predicted 196 rows (3+81+6+50+56), taking chapter table from 31,307 to 31,111. After it, DB matches plugin answer exactly for all 5 (7/7, 50/50, 57/57, 82/82, 6/6), 0 orphan read_history, 0 orphan chapter_pages, 0 orphan chapters.

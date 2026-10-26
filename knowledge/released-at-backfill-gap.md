---
type: finding
title: The released_at column was added on 20 Oct but the rows synced before it were never backfilled - the plugin was always right
description: "released_at was empty on old rows, not the plugin - the column was added (79fc8f0) but pre-existing rows were never backfilled. Closed by re-syncing the library."
tags: [goisekai, database, chapters, released_at, migration, backfill, plugins, 1manga]
generated: { by: agent/mcp, at: "2026-10-26T15:56:47Z" }
status: stable
---

# released_at was empty on old rows, not on the plugin's answer

**Not a plugin bug.** The 1manga plugin returns released_at on every chapter. The empty ones were rows in SQLite that had been synced BEFORE the released_at column existed, and nothing ever backfilled them.

## The evidence

| source | chapters | with released_at |
|---|---|
| GET /api/sandbox/plugins/1manga/chapters/martial-peak | 3,865 | 3,865 |

sqlite> SELECT COUNT(*), SUM(released_at IS NULL) FROM chapters c JOIN mangas m ON m.id=c.manga_id WHERE m.plugin_id='1manga';
2489|2489

The split is a clean time boundary:

| released_at | rows | fetched_at range |
|---|---|---|
| NULL | 2,494 | 2026-10-18 20:13 to 2026-10-20 15:58 |
| filled | 222 | 2026-10-20 17:01 to 2026-10-21 06:12 |

Commit `79fc8f0` added the column on 20 Oct - landed exactly between last NULL and first filled. **The column was added, but rows synced before it were never rewritten.**

## The fix: re-sync the library

`POST /action/sync-manga/{pluginID}/{mangaID}` (CSRF from `<meta name="csrf-token">`, header `X-CSRF-Token`) re-fetches chapter list and upserts. `released_at` in both `DO UPDATE` clauses, so date refreshes in place.

`POST /action/sync` (Update button) syncs the whole library - 519 manga, 355s, 453 re-synced (66 failed anti-bot, unrelated):
```
released_at NULL:  26,145  ->  7,913
```

Remaining NULLs are NOT a backfill gap:

| kind | rows | why |
|---|---|---|
| old rows, plugin id format changed | 1,570 | old id no longer emitted (lhtranslation slug change) |
| plugin never sends field | 4,503 | bato1, mangakatana, mangafreak, ksgroupscans, topmanhua, comix, onisaga |
| plugin sends zero time | 1,837 | mangaread/mangzio/anisascans send 0001-01-01 (site has no date, host stores NULL) |

**Rule: empty released_at = "no date the site gives", not "plugin broken".**

## Scheduler

Hourly scheduler (`SyncStaleLibrary`) only refreshes manga older than `update_stale_days` (3 days). After migration only 12 of 519 were stale - would leave the gap. Use the Update button.

## UI

`detail_chapters.lua:145` only renders a date when `c.ReleasedAt` is present and doesn't start with `0001` - NULL row simply shows no date.

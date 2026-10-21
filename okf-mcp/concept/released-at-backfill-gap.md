---
id: released-at-backfill-gap
title: The released_at column was added on 20 Oct but the rows synced before it were never backfilled - the plugin was always right
type: finding
tags: [goisekai, database, chapters, released_at, migration, backfill, plugins, 1manga]
---

# released_at was empty on old rows, not on the plugin's answer

Reported: "bro 1manga untuk status release_at nya kosong itu bug atau bukan?"

## Short answer

**Not a plugin bug.** The 1manga plugin returns `released_at` on every single
chapter. The empty ones were rows in SQLite that had been synced *before* the
`released_at` column existed, and nothing ever backfilled them.

## The evidence

The plugin's own output is complete:

| source | chapters | with released_at |
|---|---|---|
| `GET /api/sandbox/plugins/1manga/chapters/martial-peak` | 3,865 | **3,865** |
| `GET /api/sandbox/plugins/1manga/chapters/<another manga>` | 109 | **109** |

The database was not:

```
sqlite> SELECT COUNT(*), SUM(released_at IS NULL) FROM chapters c
          JOIN mangas m ON m.id=c.manga_id WHERE m.plugin_id='1manga';
2489|2489
```

The split in the database is a clean time boundary, not a random scatter:

| released_at | rows | fetched_at range |
|---|---|---|
| NULL | 2,494 | 2026-10-18 20:13 → **2026-10-20 15:58** |
| filled | 222 | **2026-10-20 17:01** → 2026-10-21 06:12 |

The commit that added the dedicated column is
`79fc8f0 fix(database): store the site's chapter upload date in its own
column`, dated **2026-10-20 23:44 +0700 = 16:44 UTC** - which lands exactly
between the last NULL row and the first filled row. The concept
`chapter-release-date-column.md` describes the change; this is the gap it left
behind: **the column was added, but the rows that had been synced before it
were never rewritten.**

Decisive test: re-syncing one manga with NULL dates through
`POST /action/sync-manga/1manga/<slug>` filled all five of its chapters at
once (`2023-07-19`, `2022-10-04`, ...) - the plugin had the dates all along.

## The fix: re-sync the library

`POST /action/sync-manga/{pluginID}/{mangaID}` (CSRF token from
`<meta name="csrf-token">` in any view, sent as `X-CSRF-Token`) re-fetches the
chapter list and upserts it. `UpsertChapter` has `released_at` in **both**
`DO UPDATE` clauses, so a re-sync refreshes the date in place.

Running it over the 50 affected 1manga library manga (one at a time - the
fetch lane is a single worker, a burst only queues behind itself) took the
count to **0 NULL out of 2,799 chapters**. The detail page now renders the
dates (`/view/manga/1manga/shadow-skill`: 92 chapters, dates visible), and
`released_at` is the site's own date while `fetched_at` is the sync time -
e.g. `released_at 2022-04-24 21:56:04` against `fetched_at 2026-10-21
06:23:58`.

## Not only 1manga

Every plugin with chapters synced before the 20 Oct migration has the same
gap. The whole library was then re-synced through the button that already
exists in the UI - `POST /action/sync` (the **Update** button on the library
page) - which is `SyncLibrary()` -> `syncMangas()`, 519 manga in one
sequential pass on the fetch lane, **355 seconds**, 453 of 519 manga
re-synced (the other 66 failed on an anti-bot challenge, which is unrelated to
the date).

```
released_at NULL:  26,145  ->  7,913
```

The remaining NULLs are **not** a backfill gap any more - they are rows the
source genuinely has no date for:

| kind | rows | why |
|---|---|---|
| old rows, plugin id format changed | 1,570 | the plugin no longer emits that id at all (e.g. lhtranslation switched from bare `chapter-53` to `<slug>:chapter-53`), so a re-sync creates the new row and leaves the old one orphaned |
| plugin never sends the field | 4,503 | bato1 2,341, mangakatana 886, mangafreak 816, ksgroupscans 305, topmanhua 110, comix 40, onisaga 5 |
| plugin sends a **zero time** | 1,837 | mangaread, mangzio, anisascans answer `"released_at": "0001-01-01T00:00:00Z"` - the site has no date, and the host stores that as NULL on purpose |
| misc | 3 | mangahub |

`manga_uno` is the same zero-time case (all 74 rows come back as
`0001-01-01T00:00:00Z`).

**Rule:** an empty `released_at` means "this row has no date the site is
willing to give", not "the plugin is broken". The backfill gap was real for
the rows that predate the 20 Oct migration and is now closed.

## The scheduler will not do this on its own

`POST /action/sync` (the Update button) syncs **every** manga in the
library, while the hourly scheduler (`StartLibraryScheduler` ->
`SyncStaleLibrary`) only refreshes manga whose `updated_at` is older than
`update_stale_days` (3 by default). After the migration only 12 of 519
library manga were stale, and 507 were fresh - so the scheduler would have
left almost all of the gap untouched. The button is the tool for this.

## Note on the UI

`internal/templates/partials/detail_chapters.lua:145` only renders a date
when `c.ReleasedAt` is present and does not start with `0001`, so a NULL row
simply shows no date - there is no placeholder to look like a bug.

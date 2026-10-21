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
gap. The ones whose build **does** emit `released_at` are backfillable the
same way (verified with `strings` on the installed binary):

| plugin | NULL rows | sends released_at |
|---|---|---|
| mangafire | 5,376 | yes |
| atsumaru | 5,703 | yes |
| mangak | 1,984 | yes |
| mangadex | 1,342 | yes |
| mangaread | 1,442 | yes |
| mangahub | 1,099 | yes |
| mangabuddy | 1,088 | yes |
| lhtranslation | 715 | yes |
| mangakatana | 886 | no |
| ksgroupscans | 305 | **no** - the source has no date |

**Rule:** an empty `released_at` means "this row was synced before the
column existed", not "the plugin does not know the date". Only the plugins
that genuinely never send the field (ksgroupscans and friends) keep NULL for
new rows too.

## Note on the UI

`internal/templates/partials/detail_chapters.lua:145` only renders a date
when `c.ReleasedAt` is present and does not start with `0001`, so a NULL row
simply shows no date - there is no placeholder to look like a bug.

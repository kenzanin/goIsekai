---
id: chapter-release-date-column
title: Chapter upload dates showed the sync day - fetched_at was overwritten and then read back as the release date
type: finding
tags: [goisekai, database, chapters, plugins, jet, migration, dates]
---

# Chapter upload dates showed the sync day

Reported as "tanggal upload chapter manga di website nya" wrong: nearly every
chapter row on a manga detail page read "20 Oct 2026" — the day the row was
last synced, not the day the site published the chapter.

## Root cause: one column carrying two meanings

The chapter list has exactly one date column, `fetched_at`, and the code
treated it as both "when we synced" and "when the site published it".

```
plugin GetChapterList  →  Chapter.ReleasedAt  (the site's own upload date)
  │
  ├─ bridge/sync.go:161   FetchedAt: c.ReleasedAt      ← the date arrives here
  │
  └─ database/chapters.go  UpsertChapter
       Chapers.FetchedAt = RawTimestamp("CURRENT_TIMESTAMP")   ← and is dropped here
```

`UpsertChapter` wrote `CURRENT_TIMESTAMP` into `fetched_at` in **both** its
INSERT branches and **both** its `DO UPDATE` clauses, and never read the
`c.FetchedAt` it had been handed. The three read paths then mapped that same
column back out as the release date:

| site | mapping |
|---|---|
| `bridge/service.go:164` | `ReleasedAt: c.FetchedAt` (inactive plugin) |
| `bridge/library_fallback.go:26` | `ReleasedAt: c.FetchedAt` (live fallback) |
| `bridge/library_fallback.go:99` | `ReleasedAt: c.FetchedAt` (cached fallback) |

So the plugin's date was computed, carried to the database, and discarded on
the way in — while the sync timestamp was stored in the one field the UI read.

**DB evidence.** `SELECT date(fetched_at), COUNT(*) FROM chapters GROUP BY 1`
returned only sync days — 2026-10-20: 11138, 10-19: 8702, 10-18: 2365,
10-17: 7889, then a long tail of 44, 3, 8, 4, 4, 196. **Zero real upload
dates were stored anywhere in the database.**

## Why not just stop overwriting fetched_at

Tempting, and it would have shown the right date. It was rejected:

- `fetched_at` would silently change meaning from "when we synced" to "when
  the site published it", with no column recording either fact separately.
- 17 of the Lua plugins send **no** `released_at` at all (atsumaru, comix,
  dynastyscan, fanfox, htmlscrape, kaliscan, madaradex, mangade, mangafreak,
  mangahere, mangakatana, mangataro, mangatown, topmanhua, valirscans,
  violetmanga, zeroscann). A reused column cannot express "the plugin knows
  no date" — the row would show some invented date instead of nothing.

So `chapters` got a **dedicated `released_at` column**, and `fetched_at` kept
its original meaning.

## The fix

**Migration 30** (`internal/database/schema.go`):

```sql
ALTER TABLE chapters ADD COLUMN released_at TIMESTAMP;
```

NULL means "the plugin knows no date" — deliberately distinct from a real
date. It is in the idempotent-column list in `migrations.go` (with
`releasedAtMigration = 30`), so a rewind-replay or a rebuilt database cannot
fail on "duplicate column".

**Write path** (`internal/database/chapters.go`):

```go
func chapterReleasedAt(t time.Time) TimestampExpression {
	s := ReleasedAtString(t)
	if s == "" {
		return TimestampExp(NULL)
	}
	return TimestampExp(String(s))
}
```

Used in both INSERT branches and both `DO UPDATE` clauses. `fetched_at` still
gets `CURRENT_TIMESTAMP`.

**Read paths** — the four mapping sites now read the dedicated column:
`bridge/service.go:164`, `bridge/library_fallback.go:26` and `:99`, and
`bridge/sync.go:161` (which now passes `ReleasedAt: c.ReleasedAt` instead of
`FetchedAt: c.ReleasedAt`). `bridge/migrate.go`'s raw-SQL insert also stores
it, so a migrated manga keeps its dates.

### The write path was probed, not assumed

`TimestampExp(String(...))` and `TimestampExp(NULL)` were run against a real
migrated database before being used, to confirm the round trip:

```
raw  a  typeof=text     value=2025-01-02 03:04:05 +0000 UTC
raw  b  typeof=null     value=<nil>
raw  c  typeof=text     value=2024-09-07 17:04:15 +0000 UTC
jet  a  title=A2  fetched_at=2025-01-02T03:04:05Z
jet  b  title=B2  fetched_at=NULL
jet  c  title=C   fetched_at=2024-09-07T17:04:15Z
```

## The jetgen trap (bit twice, worth recording)

`cmd/jetgen`'s DDL list is supposed to mirror `internal/database/schema.go`'s
`migrations` slice. It had fallen **7 ALTERs behind**: `http_profile`,
`plugins.genres`, `mangas.author`, `mangas.custom_cover`,
`mangas.reader_prefs`, `chapters.total_pages`, `chapters.is_skipped`.

Consequence: running `go run ./cmd/jetgen` produced a diff that **dropped**
`CustomCover` from the generated `mangas` table (2 insertions / 5 deletions) —
a regression, not a fix. The committed `.gen` was hand-patched at the time
(`3595f76` "stale codegen from migration 28"), so the drift was invisible
until the generator ran.

Fixing this required adding the missing ALTERs to `cmd/jetgen/main.go`
**first**, then regenerating. The regeneration then repaired the stale
codegen as a side effect: `plugins.http_profile`, `plugins.genres`,
`mangas.author` and `mangas.reader_prefs` came back.

**Rule:** before regenerating `.gen`, diff the generator's DDL against the
migrations slice. A generator that has fallen behind produces a diff that
*removes* real columns.

`internal/database/migration_storage.go` needed no change: the storage
migration is index 22, before the new index 30, so it never rebuilds a
`chapters` table that already has `released_at`.

## Verification

Five regression tests across two levels (`internal/database` and
`internal/bridge`) — the database tests pin the storage behaviour, the bridge
tests pin the read sites the detail page actually goes through:

- a release date survives the upsert round trip
- a plugin with no date leaves `released_at` NULL (not the sync time)
- a re-sync refreshes `fetched_at` without discarding `released_at`
- the cached chapter list reports the release date
- the inactive-plugin chapter list reports the release date

Live, on weebcentral after `POST /action/sync-manga`:

```
75 chapters, 75 with released_at
released_at histogram: 2024-09-07 → 66, 2024-11-11 → 9
detail page: "Sep 8, 2024" ×41, "Nov 11, 2024" ×9
```

Before the fix that page showed "20 Oct 2026" on every row.

### Timezone

The site's own JS (`script.min.js`, `displayCommentTime` + dayjs) formats the
`datetime` attribute in the **visitor's local** zone and renders
`MMM D, YYYY` — the same shape the host's `formatDate` produces. So the host
and the site agree, and no day-shift is introduced by storing the date in UTC.

## Follow-up not done

Existing rows keep `released_at` NULL until their manga is re-synced. There
is no periodic library sync — `POST /action/sync` (all) and
`/action/sync-manga/{pluginID}/{mangaID}` (one) are the only triggers, so
the dates fill in as the user syncs.

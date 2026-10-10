# Design: offline-first-fallback

## Context

The reader already survives offline via `readerChapters` (cached-first, live
second). The detail page calls only the live path. Separately,
`pluginmanager.Manager.SetDB` is dead code — `main.go` never calls it, so the
`plugin_cache` layer inside `GetMangaDetail`/`GetChapterList` never activates
and the table stays empty. The four optimization items from
OPTIMIZATION_OFFLINE_FIRST.md that survived audit (PRAGMAs, one index, longer
chapter TTL, image size prune) ride along here because they share the offline
goal and touch the same files.

## Goals / Non-Goals

- Goals: detail page renders offline from persisted data; plugin_cache actually
  functions; cheap SQLite tuning; bounded image cache disk usage.
- Non-Goals: Ristretto or any in-memory query cache (SQLite is fast enough for
  single-user); background prefetch scheduler (StartLibraryScheduler already
  exists); offline stats endpoint; env-var configuration (project uses
  goisekai.ini with hot reload); network-detection heuristics (fallback is
  error-driven, no probing).

## Decisions

### D1: Detail fallback lives in the handler, not the bridge

`viewMangaDetail` already receives a typed error only for challenges; generic
failures return nil. Rather than teaching the bridge a third fetch mode, the
handler retries with the existing `CachedMangaAndChapters` and sets a
`CachedData: true` flag in the template data when it succeeds. This mirrors
reader.go's structure one level up, keeps bridge semantics unchanged, and the
challenge path (anti-bot interstitial) stays exactly as-is. The fallback also
fills author/genres/cover-dim from enrichment tables the same way the live path
does, so the page looks complete.

- Alternative considered: make `GetMangaDetails` itself fall back. Rejected:
  it already has a `cachedMangaFallback` branch for plugin failure; the blank
  page comes from the handler's nil check, so fixing the handler is the
  minimal, honest change.

### D2: Cache-data notice is server-rendered, not JS

One template conditional in `views/detail`, styled like the existing muted
"updated X ago" text. No new JS, no localStorage, no network-status events —
offline browsers can't be trusted to report their own status anyway.

### D3: Activate SetDB with per-function TTLs, not a global override

`main.go` calls `mgr.SetDB(db, detailTTL)` where detailTTL comes from the
existing `cache_ttl_hours`. Chapter lists get their own TTL via a new
`SetChapterCacheTTL` setter fed from a new `chapter_cache_ttl_hours` config key
(default 168). Rationale: chapter lists change rarely (weekly for ongoing
titles), detail metadata even more rarely, but a stale detail page is more
noticeable than a stale chapter list — so the longer TTL belongs to chapters.
`config_defaults.go` gains the key; `goisekai.ini` picks it up on next
auto-generation; hot reload refreshes it alongside `cache_ttl_hours`.

### D4: PRAGMAs inline in database.Open's DSN

`_cache_size=-64000&_mmap_size=268435456&_sync_mode=NORMAL` appended to the
existing DSN string. No config surface: these are safe-for-CGO=0 defaults, not
tuning knobs anyone asked for. WAL + NORMAL trades a last-transaction window on
power loss for materially less fsync pressure; acceptable for a reader app
whose data is re-fetchable.

### D5: Size-based image prune in the existing maintenance flow

The startup/periodic maintenance path already exists (`startMaintenance`).
Pruning rides the same tick: walk `<cache_dir>/images`, sum sizes, delete
oldest-first past `max_cache_gb` (new `[maintenance]` key, default 2, 0 = off).
No watcher, no separate goroutine, no XAttr/ATime dependence (mtime only —
reading a cached page updates it via the L1 write path, keeping "recently read"
files safe enough). Walk cost on a 2 GB cache is a few hundred ms once per
maintenance tick, not per request.

### D6: Index only where a query actually scans

`idx_plugin_cache_expires` on `plugin_cache(expires_at)`: `CleanExpired` runs
`DELETE ... WHERE expires_at <= now` and `GetCache` filters on the same column
today. The partial `is_read` index and `read_history` index from the original
doc are dropped — tables are small and the columns near-non-selective.

## Risks / Trade-offs

- [Longer chapter TTL hides new chapters for up to 7 days] → the existing
  SyncManga/pull-to-refresh path deletes and refetches the chapter list on
  explicit sync, bypassing cache; staleness only affects background browsing.
- [D4 NORMAL sync can lose the last transaction on power loss] → all data is
  re-fetchable from sources; no user-authored data lives in SQLite except
  progress, which loses at most one page-turn.
- [Prune could delete pages mid-read] → mtime updates on write, and the prune
  target is over-limit overflow (oldest first); a chapter being read now was
  written seconds ago and sits at the head of the keep-list.

## Migration Plan

Single new index in the next migrations slot. No data migration. Config keys
absent from existing INI files fall back to defaults — no migration for INI.

## Open Questions

- None. Scope deliberately excludes everything the audit showed already exists.

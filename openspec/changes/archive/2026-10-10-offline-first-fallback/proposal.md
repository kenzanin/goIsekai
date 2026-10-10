## Why

Opening a manga detail page while offline fails with a blank 502 page, even though
everything the page needs is already persisted: `mangas` carries title, cover_url,
description and status (12,902 chapter rows and 214 library entries on the dev
machine), covers are on the image disk cache, and enrichment tables hold the rest.
The reader survives offline because `readerChapters` tries `CachedMangaAndChapters`
before touching the plugin; the detail page has no such protection — it calls the
live path and returns 502 on any generic error.

Underneath that gap sits a wiring bug: `pluginmanager.Manager.SetDB` (and
`SetCacheTTL`) are never called from production code, so the plugin_cache layer in
`GetMangaDetail`/`GetChapterList` is dead code, the `plugin_cache` table stays
empty, and `cache_ttl_hours` in goisekai.ini has no effect. Every detail page load
re-invokes the plugin even when a fresh cached response exists.

An earlier offline-first plan (OPTIMIZATION_OFFLINE_FIRST.md) proposed five
priorities. Codebase audit shows most already exist (cache-first chapter lists,
background library sync, offline reader fallback) or are not justified (Ristretto
dependency, env-var config against the project's INI convention, offline stats
endpoint). This change takes only the four items with real value and the detail
fallback fix.

## What Changes

- The manga detail page falls back to the persisted copy when the plugin is
  unreachable: `viewMangaDetail` renders from `CachedMangaAndChapters` (same
  pattern the reader already uses) instead of returning 502. The page shows a
  quiet "offline / cached data" notice when served from fallback.
- `main.go` wires `mgr.SetDB(db, ttl)` using the existing `cache_ttl_hours` config
  value, activating the plugin_cache layer (cache-first GetMangaDetail and
  GetChapterList, 24h default TTL).
- SQLite PRAGMAs in `database.Open` gain `_cache_size=-64000`, `_mmap_size=268435456`,
  and `_sync_mode=NORMAL` (WAL + NORMAL is the standard durable-enough pairing).
- A new migration adds `idx_plugin_cache_expires` so the periodic `CleanExpired`
  sweep stops full-scanning `plugin_cache`.
- Plugin response cache TTL for chapter lists defaults to 7 days instead of 24
  hours (`chapter_cache_ttl_hours` config key, default 168), extending offline
  usability; detail responses keep `cache_ttl_hours`.
- The image cache gains a size-based eviction: when the cache directory exceeds
  `max_cache_gb` (new `[maintenance]` key, default 2 GB), oldest-modified files are
  pruned until under the limit, checked opportunistically after downloads.

## Capabilities

### Modified
- `bridge` — detail page renders from persisted data when the plugin is
  unreachable; plugin responses are cached in the database with per-function TTLs.
- `db-maintenance` — image cache directory is pruned to a configured size limit.

### New
- (none; all behavior extends existing capabilities)

## Impact

- Code: `internal/httpserver/views_manga.go` (fallback + notice), `internal/bridge`
  (expose cached-only fetch for the handler), `cmd/goisekai/main.go` (SetDB wiring),
  `internal/database/db.go` (PRAGMAs), `internal/database/schema.go` (one index),
  `internal/config` (two keys), `internal/pluginmanager` (chapter TTL override),
  `internal/bridge/image_cache.go` / `maintenance` (size-based prune).
- Tests: fallback render test, SetDB wiring test (cache hit on second call), prune
  test with temp dir, PRAGMA assertion in an existing db test.
- No schema changes beyond one index. No breaking changes. No new dependencies.

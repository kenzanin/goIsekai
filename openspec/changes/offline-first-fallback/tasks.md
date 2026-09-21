# Tasks: offline-first-fallback

## 1. Detail page offline fallback

- [x] 1.1 Add cached-only detail fetch to the bridge: reuse `CachedMangaAndChapters` + fill author (StoredAuthor), genres (GetMangaGenres), cover dim, alt titles/summaries/categories/related the same way `buildMangaDetailData` does; expose as `CachedDetailData(pluginID, mangaID)` on AppService.
- [x] 1.2 In `viewMangaDetail` (internal/httpserver/views_manga.go): on non-challenge error from `GetMangaDetails`, call the cached fetch; on success set `"CachedData": true` in template data; on failure keep the existing 502.
- [x] 1.3 Render a muted "cached data — offline" notice in `views/detail` template when `CachedData` is true; verify it does not appear on normal loads.
- [x] 1.4 Test: bridge tests in `internal/bridge/service_test.go` (`TestGetMangaDetailsNilManagerFallback`, `TestGetMangaDetailsCacheMiss`) cover the offline fallback behavior; handler tests would require stub AppService infrastructure not currently available.

## 2. Activate plugin_cache wiring

- [x] 2.1 `cmd/goisekai/main.go`: after `database.Open`, call `mgr.SetDB(db, detailTTL)` where detailTTL derives from `cfg.CacheTTLHours`; also call the new chapter-TTL setter from 2.2.
- [x] 2.2 `internal/pluginmanager`: add `SetChapterCacheTTL(ttl time.Duration)`; use it for `GetChapterList` cache writes/reads while `cacheTTL` continues to govern `GetMangaDetail`.
- [x] 2.3 `internal/config`: add `ChapterCacheTTLHours` (key `chapter_cache_ttl_hours`, default 168) to defaults, parser, and the INI writer list.
- [x] 2.4 Test: manager test with a real temp DB — first `GetChapterList` call invokes plugin, second call within TTL does not (plugin call counter via stub), and a chapter list written 25h ago (insert with backdated `expires_at`) is still served when TTL is 168h.

## 3. SQLite tuning

- [x] 3.1 `internal/database/db.go`: append `_cache_size=-64000`, `_mmap_size=268435456`, `_sync_mode=NORMAL` to the DSN.
- [x] 3.2 Test (extend existing db test): open a temp DB and assert `PRAGMA mmap_size` returns 268435456 and `PRAGMA synchronous` returns 1 (NORMAL).

## 4. plugin_cache index

- [x] 4.1 `internal/database/schema.go`: append migration `CREATE INDEX IF NOT EXISTS idx_plugin_cache_expires ON plugin_cache(expires_at)`.
- [x] 4.2 Verify via existing migration test run: fresh temp DB gets the index; an old DB migrates without error (run `just test ./internal/database/`).

## 5. Image cache size prune

- [x] 5.1 `internal/config`: add `MaxCacheGB float64` (key `max_cache_gb`, default 2, in `[maintenance]`) to defaults, parser, INI writer.
- [x] 5.2 `internal/bridge`: add `PruneImageCache(maxBytes int64)` — walk `<cacheDir>/images`, sum file sizes, delete oldest-mtime files until at/below limit; log freed bytes; no-op when limit <= 0.
- [x] 5.3 Wire into `startMaintenance` tick: read `MaxCacheGB` each tick (same re-read pattern as `updateStaleDays`) and call the prune.
- [x] 5.4 Test: temp dir with 3 files (oldest/newest by explicit mtime), prune to a limit that forces deleting only the oldest; assert the newest two survive and the oldest is gone.

## 6. Verification

- [x] 6.1 `just build` and `just test` green.
- [ ] 6.2 Live offline check: start server, request detail page for a library manga, confirm online render; block network for the server process (or point the plugin at an unreachable host), reload the detail page, confirm cached render with the notice; confirm cover still loads from disk cache.
- [ ] 6.3 Confirm `plugin_cache` table accumulates rows after a detail load (previously always empty), and a second load logs a cache hit instead of a plugin invoke.
- [ ] 6.4 Reload goisekai.ini with `max_cache_gb` tiny, drop a large file into the images dir, run maintenance, confirm deletion.

# Tasks: plugin-perf-optimizations

## Group 1 — Connection Pooling (foundation, no deps)

- [x] 1.1 Configure `hostnet.Proxy` with custom `*http.Transport`: `MaxIdleConnsPerHost=6`, `IdleConnTimeout=90s`, `TLSHandshakeTimeout=10s`, `DisableKeepAlives=false`. Replace `http.DefaultTransport` usage.
- [x] 1.2 Add connection pool metrics logging (idle connections, active connections) at debug level.
- [x] 1.3 Test: verify connection reuse by making 3 sequential requests to same host and checking transport stats.

## Group 2 — Plugin Caching (depends on nothing)

- [x] 2.1 Add migration: `plugin_cache` table with columns `id TEXT PRIMARY KEY, plugin_id TEXT, manga_id TEXT, function_name TEXT, response TEXT, cached_at TIMESTAMP, expires_at TIMESTAMP`. Unique index on `(plugin_id, manga_id, function_name)`.
- [x] 2.2 Implement `internal/database/cache.go`: `GetCache(pluginID, mangaID, funcName)`, `SetCache(pluginID, mangaID, funcName, response, ttl)`, `DeleteCache(pluginID, mangaID)`, `DeleteCacheByPlugin(pluginID)`, `CleanExpired()`.
- [x] 2.3 Add cache layer in `Manager.call()`: check cache before plugin invocation, store after successful invocation. Cache only `GetMangaDetail` and `GetChapterList` responses.
- [x] 2.4 Add `cache_ttl_hours` config option in `[maintenance]` section (default 24).
- [x] 2.5 Add background goroutine to clean expired cache entries every hour.
- [x] 2.6 Invalidate cache on plugin uninstall (`UnloadPlugin`).
- [x] 2.7 Test: cache hit returns same data, cache miss triggers plugin, expired cache triggers refresh.

## Group 3 — Batch ABI (depends on nothing)

- [x] 3.1 Add `GetMangaDetailWithChaptersFunc` constant to `pkg/types/abi.go`.
- [x] 3.2 Implement `Manager.GetMangaDetailWithChapters(pluginID, mangaID)`: try batch function first, fallback to separate calls.
- [x] 3.3 Update `bridge.GetMangaDetails()` to use batch function when available.
- [x] 3.4 Test: batch function called when available, fallback when not, error handling.

## Group 4 — Preconnect (depends on connection pooling)

- [x] 4.1 Implement `hostnet.Proxy.Preconnect(host string)` method: HEAD request with 5s timeout.
- [x] 4.2 Track known hosts in `app_data/known_hosts.json`. Add/remove on plugin use.
- [x] 4.3 On plugin lazy-load, extract primary host and trigger preconnect (background, non-blocking).
- [x] 4.4 On startup, load known hosts and preconnect with concurrency limit 4.
- [x] 4.5 Test: preconnect warms connection, subsequent request is faster, failure is graceful.

## Group 5 — Integration & Polish

- [x] 5.1 Update `goisekai.ini` with new config options: `cache_ttl_hours`, `preconnect_enabled`.
- [x] 5.2 Add cache stats to `/api/stats` endpoint (hit rate, total entries, memory usage).
- [x] 5.3 Run full test suite, fix any regressions.
- [ ] 5.4 Browser test: verify cache reduces detail page load time on repeat visits.

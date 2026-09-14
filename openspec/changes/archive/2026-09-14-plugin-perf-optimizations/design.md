## Architecture

### Plugin Response Caching

**Storage**: New `plugin_cache` table in SQLite with columns: `id TEXT PRIMARY KEY, plugin_id TEXT, manga_id TEXT, function_name TEXT, response TEXT, cached_at TIMESTAMP, expires_at TIMESTAMP`. Composite unique index on `(plugin_id, manga_id, function_name)`.

**Cache layer**: Insert caching logic in `Manager.call()` between plugin dispatch and return. Before invoking the plugin, check cache for a non-expired entry. After successful invocation, store the response. The cache check and store are transparent to callers.

**Staleness**: Default TTL of 24 hours. Stale entries served immediately with a background goroutine refreshing. Configurable via `goisekai.ini` `[maintenance]` section.

**Invalidation**: Manual refresh via existing "Update" button or explicit refresh endpoint. Cache entries deleted on plugin uninstall.

### Batch ABI

**New function**: `GetMangaDetailWithChapters(mangaID string) (string, error)` in `pkg/types/abi.go`. Response shape: `{"detail": Manga, "chapters": [Chapter]}`.

**Host dispatch**: `Manager.GetMangaDetailWithChapters()` attempts the batch function first. If the plugin doesn't export it, falls back to separate `GetMangaDetail()` + `GetChapterList()` calls. Detection via plugin metadata or try-catch on first invocation.

**Plugin support**: Existing Lua/JS plugins continue working via fallback. New plugins can optionally implement the batch function for performance.

### Connection Pooling

**Implementation**: Replace `http.DefaultTransport` in `hostnet.Proxy` with a custom `*http.Transport` configured with:
- `MaxIdleConnsPerHost: 6` (matches browser behavior)
- `IdleConnTimeout: 90s`
- `DisableKeepAlives: false`
- `TLSHandshakeTimeout: 10s`

**Pool management**: Go's `net/http` transport already manages connection pooling internally. No custom pool code needed — just proper Transport configuration.

### Preconnect

**Trigger**: On plugin lazy-load, extract the primary host from the plugin's configured URL patterns. Open a HEAD request to warm the connection.

**Known hosts**: Persist recently-used hosts in a simple JSON file (`app_data/known_hosts.json`). Load at startup, connect to each with concurrency limit of 4.

**Concurrency**: Use `semaphore.Weighted` with limit 4 for preconnect goroutines. Timeout per connection: 5 seconds.

## Data Flow

```
User request → Manager.GetMangaDetails()
  → Check plugin_cache (hit? return cached)
  → Manager.GetMangaDetailWithChapters() (if batch available)
    → Plugin.GetMangaDetailWithChapters()
    → Cache response
    → Return
  → Fallback: Manager.GetMangaDetail() + Manager.GetChapterList()
    → Each checks cache, invokes plugin if miss, caches result
    → Return combined result
```

## Files to Modify

| File | Change |
|------|--------|
| `internal/pluginmanager/api.go` | Cache layer in `call()`, batch function dispatch |
| `internal/pluginmanager/lifecycle.go` | Preconnect on plugin load |
| `internal/hostnet/request.go` | Custom Transport with pooling config |
| `internal/database/schema.go` | `plugin_cache` table migration |
| `internal/database/cache.go` | Cache CRUD operations |
| `pkg/types/abi.go` | `GetMangaDetailWithChapters` constant |
| `internal/bridge/library.go` | Use batch function in `GetMangaDetails()` |
| `internal/config/config.go` | Cache TTL and pool settings |

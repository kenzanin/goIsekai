## Why

Plugin calls dominate page load latency. Every `Search()`, `GetMangaDetail()`, and `GetChapterList()` triggers HTTP requests to external sites via `hostnet.Proxy`, taking 50-500ms each. For library views with 24 manga, detail+chapter fetches can take 5-10 seconds total. We can eliminate redundant HTTP calls for cached manga, batch related fetches into single plugin invocations, and warm HTTP connections proactively.

## What Changes

- **Result caching**: Cache plugin detail/chapter responses in the database. Skip HTTP calls when manga data is already cached and fresh. Cache invalidation via manual refresh or staleness threshold.
- **Batch ABI**: New `GetMangaDetailWithChapters(mangaID)` plugin function that returns both detail and chapter list in one call, halving HTTP roundtrips for detail page loads.
- **HTTP connection pooling**: Add persistent connection pool to `hostnet.Proxy` with per-host limits, keep-alive, and connection reuse across plugin calls.
- **Preconnect**: Open HTTP connections to popular/active plugin sites at startup so first user request skips TCP+TLS handshake.

## Capabilities

### New Capabilities
- `plugin-caching`: Cache plugin responses in DB, serve from cache when fresh, refresh on demand or staleness
- `batch-abi`: Combined plugin function returning detail+chapters in single invocation
- `connection-pooling`: Persistent HTTP connection pool in hostnet proxy with per-host limits
- `preconnect`: Proactive HTTP connection warming to plugin sites at startup

### Modified Capabilities
- `plugin-abi`: Add `GetMangaDetailWithChapters` to the plugin ABI contract

## Impact

- `internal/pluginmanager/api.go`: Add cache layer, batch function dispatch
- `internal/pluginmanager/lifecycle.go`: Preconnect on plugin load
- `internal/hostnet/request.go`: Connection pool implementation
- `internal/database/`: Cache storage schema (new table or column additions)
- `pkg/types/abi.go`: New ABI function signature
- All existing Lua/JS/Go/Yaegi plugins: Optional batch function implementation

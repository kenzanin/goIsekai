# Proposal

## Why

A manga whose cover and chapter pages were cached stops showing images, online or offline. Two independent causes lock together. The image cache is pruned oldest-write-first at the `max_cache_gb` cap, with no regard for library membership or for what was read recently, so cached files of a library manga age out. And the refill fails: MangaDex now redirects browser-like `User-Agent` values (the host sends Chrome by default) to an edge bot challenge, the host follows the redirect, receives an HTML page, and rejects it as a corrupt image three times before returning 502. The same request with a non-browser `User-Agent` succeeds, so the failure is the host's identity choice, not the site being closed. See `knowledge/bugs/offline-image-cache-pruned-and-mangadex-challenge.md`.

## What Changes

- Plugins can declare an ordered `user_agents` list in `plugin.json`. The first entry is used from the first request; later entries are tried only when the site blocks the current one. The winning entry is remembered for the plugin. The literal `default` stands for the host's configured User-Agent.
- The image fetch treats a 2xx HTML response to an image request as a block, not as a corrupt image: it walks the plugin's remaining User-Agent entries and TLS profiles, and surfaces a challenge error when all fail, instead of burning three identical retries.
- The image cache prune becomes recency-aware and library-aware: reading a cached file counts as use, and files belonging to library manga are evicted only after everything else.
- The MangaDex plugins (`lua` and `wasm`) declare a non-browser User-Agent first.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `host-network`: adds the plugin-declared User-Agent ladder and block detection for non-image responses to image requests.
- `db-maintenance`: the image cache size limit requirement gains recency-by-use eviction and library protection.

## Impact

- `pkg/types` (`PluginMeta`), `internal/pluginmanager` (discovery and lifecycle push the list to the proxy), `internal/hostnet` (request path, ladder, pins), `internal/bridge` (`fetchImageWithRetry`, `cachedImage`), `cmd/goisekai/prune.go`.
- `examples/plugins/lua/mangadex/plugin.json` and `examples/plugins/wasm/mangadex_odin/plugin.json`, plus the installed copy under `app_data/plugins/mangadex`.
- No new dependency and no database migration. Behaviour changes only for plugins that declare `user_agents`; every other plugin keeps the host default.

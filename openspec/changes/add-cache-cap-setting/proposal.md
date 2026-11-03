# Proposal

## Why

The image cache cap (`max_cache_gb`, default 2) is already honoured and hot-reloaded, but it can only be changed by editing `goisekai.ini` by hand. The Settings page shows how big the cache is and offers "Clear all cached images", yet gives no way to set the cap that decides what the prune deletes. A library that outgrows 2 GB has its cached covers and pages evicted (see `knowledge/bugs/offline-image-cache-pruned-and-mangadex-challenge.md`), and the only remedy is a file edit. Lowering the cap also has no visible effect until the next restart, because the prune runs only at startup and on the maintenance tick.

## What Changes

- The Settings page gets an "Image cache limit" field (GB) beside the existing "Cached images" figure, saved through the existing `save-settings` action.
- The value is validated: a non-negative number; `0` means "never prune by size". Invalid input leaves the stored value unchanged, as other numeric settings do.
- A saved cap takes effect without a restart, including lowering it: when the hot-reloaded cap is lower than the cache's current size, a prune runs off the request path.
- The page states that `0` disables size-based pruning.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `config-persistence`: the Settings page exposes the image cache limit and validates it on save.
- `db-maintenance`: a changed cap is applied by pruning promptly instead of waiting for the next startup or maintenance tick.

## Impact

- `internal/httpserver` (`handleSaveSettings`), `internal/templates/views/settings.lua`, `cmd/goisekai/maintenance.go` (the config-reload callback), `internal/config` tests (the round-trip already carries `max_cache_gb`).
- No new setting key, no migration, no new endpoint and no new dependency. Independent of `image-fetch-resilience`, which changes how the prune orders files, not how the cap is set.

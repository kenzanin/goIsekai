# Tasks

## 1. Plugin-declared User-Agent ladder

- [x] 1.1 Add `UserAgents []string` (`user_agents`) to `types.PluginMeta` and cover it in `pluginjson_test.go`
- [x] 1.2 Add `Proxy.SetUserAgents(pluginID, list)` (drop empty entries, treat `default` as the host value, clear on empty list) and the in-memory `uaPins` map, cleared together with the TLS pin in `ClearPin`
- [x] 1.3 Push the list from plugin discovery/lifecycle next to `SetHTTPProfiles`, so it is set before the first request and refreshed on reload
- [x] 1.4 Resolve the User-Agent in one helper used by `doRequestProfile` and `doRequestStd` with the order verify-seeded UA, per-request header, pin or first entry, host default; keep `Sec-CH-UA` in lockstep
- [x] 1.5 Extend `tryLadder` to walk untried User-Agent entries on the current profile before the TLS profiles, pinning the first entry that clears the block
- [x] 1.6 Tests in `internal/hostnet`: first entry on request one, fallback to the second on a block, pin reused afterwards, every entry blocked returns the challenge error, verify UA wins, per-request header wins, no `Sec-CH-UA` for a non-Chromium value, plugin with no list unchanged

## 2. Non-image response is a block on the image path

- [x] 2.1 Add `Proxy.RequestImageContext` that wraps `RequestContext`, treats a 2xx HTML response (by content type or body shape) as a block, drives the fallback from task 1.5, and returns the challenge error when exhausted
- [x] 2.2 Switch `fetchImageWithRetry` to it, keep the corrupt-bytes retry for non-HTML bodies, and make sure an HTML body is never written to L1 or L2
- [x] 2.3 Tests in `internal/bridge`: a 200 HTML interstitial moves to the next User-Agent, an exhausted ladder yields a challenge error (not `invalid image data`), a truncated JPEG keeps the old retry, and no cache file is created for a blocked response
- [x] 2.4 Check the image handler's error mapping so a challenge error is logged as a block, not as an upstream failure with a stack of identical warnings

## 3. Recency- and library-aware cache prune

- [x] 3.1 In `cachedImage`, touch the served file's mtime with `os.Chtimes` when it is older than one hour, ignoring errors
- [x] 3.2 Add a bridge method that returns the library protection predicate: cover base paths (cover and alternative covers through `diskCachePath`) and the `images/<plugin>/<mangaID>/` directories of library manga
- [x] 3.3 Change `pruneImageCache` to take the predicate, delete unprotected files by mtime first, then protected files by mtime only if still over the cap, and log how much of each was freed
- [x] 3.4 Update the startup and maintenance callers in `cmd/goisekai` to pass the predicate
- [x] 3.5 Tests extending `TestPruneImageCache`: unprotected files go first regardless of age, library files survive while others remain, the cap is still met when only library files remain, a touched old file outlives an untouched newer one, `max_cache_gb` 0 deletes nothing

## 4. MangaDex plugins

- [x] 4.1 Add `"user_agents": ["goIsekai/1.0 (personal reader)", "default"]` to the `plugin.json` of `examples/plugins/lua/mangadex` and `examples/plugins/wasm/mangadex_odin`
- [x] 4.2 Refresh the installed copy under `app_data/plugins/mangadex` from the wasm plugin and confirm the host reads the new field after a restart

## 5. Verification

- [x] 5.1 `go test ./internal/hostnet/ ./internal/bridge/ ./internal/pluginmanager/ ./cmd/goisekai/ -count=1`, `just check` with 0 issues, `just build`
- [x] 5.2 Restart the host and open `mangadex/a4a10d24-149f-4f7b-8fc1-2784d377fb08`: the cover and a chapter's pages load, the log shows no `image corrupt`, and `curl` with a browser User-Agent against the cover URL is still the reproduction of the old failure
- [x] 5.3 Confirm a plugin with no `user_agents` still sends the configured User-Agent, and that a cached cover still renders with the network disabled
- [x] 5.4 Record the outcome in `knowledge/bugs/offline-image-cache-pruned-and-mangadex-challenge.md` and add the "browser UA triggers, empty UA gives 400" finding

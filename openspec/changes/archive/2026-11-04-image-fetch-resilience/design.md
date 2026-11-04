# Design

## Context

Observed in `knowledge/bugs/offline-image-cache-pruned-and-mangadex-challenge.md` and re-checked while planning:

- Every plugin request, and every image the host fetches for a plugin, goes through `hostnet.Proxy.RequestContext(pluginID, ...)`. `buildHeaders` applies the host default `User-Agent`; a per-plugin override exists (`uaOverrides`) but is only seeded by the verify flow (`SetVerifyCookies`), and it wins over everything, including a per-request header.
- TLS profile hints already come from `plugin.json` (`SetHTTPProfiles` fills `Proxy.hints`) and are tried by `tryLadder` only after `isChallengeResponse` (403/503 plus Cloudflare markers) or `isWafBlock` matches. MangaDex's challenge is a 302 to `/edge/challenge`, which `tls-client` follows, so the caller sees 200 `text/html` titled "Browser verification". Neither detector matches it, and `types.HTTPResponse` carries no final URL.
- `fetchImageWithRetry` validates the body with `validateImageFull`; on failure it logs `image corrupt` and retries the same request three times, then returns `invalid image data`.
- `pruneImageCache` (`cmd/goisekai/prune.go`) walks `images/`, sorts by mtime and deletes until under cap. `cachedImage` reads without touching mtime. Covers live at `images/<plugin>/library/<sha256[:8]>.<ext>` keyed by URL path and canonical query (`diskCachePath`); chapter pages live under `images/<plugin>/<mangaID>/<chapterID>/`.
- Measured against `curl`: browser User-Agents (Chrome, Firefox, Safari) get the challenge on both `uploads.mangadex.org` and `api.mangadex.org`; `curl/x`, `Go-http-client/2.0` and a custom `goIsekai/1.0 (...)` pass; an empty `User-Agent` returns 400. TLS impersonation of Chrome (`curl_chrome131`) is challenged too, so the trigger is the claimed browser identity, not the TLS fingerprint.

## Goals / Non-Goals

**Goals:**
- A plugin can state which `User-Agent` its site accepts, with fallbacks, in the metadata it already ships.
- A block that arrives as a 2xx HTML page is recognised as a block on the image path and walks the ladder instead of looping.
- The prune stops evicting files that are in use or that belong to the library before it evicts anything else.

**Non-Goals:**
- No bypass of a challenge by imitating a browser. The ladder offers honest identities first and the existing CDP and verify flows remain the escalation.
- No MangaDex OAuth. The personal client credentials are not needed for the public read endpoints used here.
- No database migration and no new setting. `max_cache_gb` keeps its meaning.
- No change to `types.HTTPRequest` (the plugin ABI).
- Not a general "detect a challenge in any 2xx HTML" rule for plugin API calls, which cannot tell an HTML page from a legitimately HTML source.

## Decisions

### 1. `user_agents` mirrors `http_profiles`, with the first entry active from the start

`PluginMeta` gains `UserAgents []string` (`user_agents`). Discovery pushes it to the proxy next to the TLS hints (`SetUserAgents(pluginID, list)`, replacing or clearing on reload). `doRequestProfile` and `doRequestStd` resolve the User-Agent through one helper that implements the precedence in the spec.

The first entry is active immediately rather than only after a failure. The alternative (default first, list only as fallback) costs a blocked round trip on every first request after start, and depends on the detector recognising the block, which is exactly what fails today. The literal `default` keeps the host value reachable in the list; `none` is not offered because an empty value is rejected by the very site that motivated this.

Alternative rejected: a per-host User-Agent table in `hostnet`. It would fix MangaDex but is a host-side dictionary of site quirks, against the house rule that the plugin adapts to the host and declares its own needs.

### 2. The winning entry is remembered in memory only

`uaPins map[pluginID]string` lives in the proxy and is cleared by the same action that clears the TLS pin. With the first entry active from the start, a remembered fallback only matters when the first entry stops working, so re-learning it after a restart costs one extra request. Persisting it would add a column and a migration for that. ponytail: in-memory only; upgrade path is a `plugins.ua_pin` column wired like `http_profile` if restarts prove costly.

### 3. UA fallback runs before TLS profiles, with the current profile

On a block, `tryLadder` first walks the remaining User-Agent entries on the current profile, then the existing TLS ladder. Walking the product (UAs times profiles) would turn a block into up to 8 times N probes. The two dimensions are independent in the one case we have evidence for, so they are tried one after the other.

### 4. Image-aware block detection lives in a new proxy entry point, not in the ABI

`fetchImageWithRetry` needs "this is an image request" while `hostnet` does not know that. A new `Proxy.RequestImageContext` wraps `RequestContext`: after a 2xx it checks the response `Content-Type` and body prefix, and if it is HTML it reports a block and drives the fallback in decision 3. The bridge calls it instead of `RequestContext`. A genuinely corrupt non-HTML body still returns normally and keeps the existing retry in `fetchImageWithRetry`. This avoids adding an `expect` field to `HTTPRequest`, which would be a plugin ABI change.

Detection keys on the HTML shape of the body rather than on a MangaDex marker string, so another site that serves an interstitial with a 200 is covered without a new rule per site.

### 5. Recency by touching mtime on a disk-cache hit

`cachedImage` calls `os.Chtimes` on the file it served when its mtime is older than one hour. The prune keeps sorting by mtime, so it needs no format change and no atime support (many mounts use `noatime`). The one-hour threshold bounds the extra writes for a reader scrolling through hundreds of cached pages. A failed `Chtimes` is ignored: the file is still served.

### 6. Library protection is a predicate supplied to the prune

`pruneImageCache` takes a `protected func(path string) bool`. The bridge builds it once per run from the database: for each library manga, the cover base path of its `cover_url` and of its alternative covers through `diskCachePath`, and the directory `images/<plugin>/<mangaID>/`. Unprotected files are sorted by mtime and deleted first; if the total is still over the cap, protected files are deleted by mtime, so the cap is still honoured when the library alone exceeds it. Computing covers through `diskCachePath` keeps the mapping in one place instead of re-implementing the hash in the prune.

### 7. MangaDex plugin metadata

Both `examples/plugins/lua/mangadex/plugin.json` and `examples/plugins/wasm/mangadex_odin/plugin.json` declare `"user_agents": ["goIsekai/1.0 (personal reader)", "default"]`. The installed copy under `app_data/plugins/mangadex` is refreshed from the wasm one.

## Risks / Trade-offs

- MangaDex may later block a custom User-Agent too. The ladder then falls through to `default` and the CDP engine; no worse than today.
- The HTML-body rule could misfire on a source whose image CDN legitimately returns HTML for a missing file with a 200. It only changes behaviour from "retry identically then fail" to "try the next identity then fail", so a false positive costs a few requests, not correctness.
- The in-memory pin forgets a learned fallback on restart (decision 2).
- Library protection makes the cache able to sit at the cap with mostly library files, leaving less room for browsing thumbnails. Browsing files are evicted first, which is the intended trade.
- `Chtimes` on every cached read is an extra metadata write; the one-hour threshold bounds it.
- Claiming a non-browser identity may be refused by sources that require a browser User-Agent. Only plugins that opt in via `user_agents` are affected.

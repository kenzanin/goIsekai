---
type: Bug
title: "Cached cover and chapter images vanish: FIFO prune plus MangaDex edge challenge blocks re-fetch"
description: "Cached images disappear because pruneImageCache evicts oldest-written files first at the 2 GB cap, and re-fetching fails since MangaDex redirects browser User-Agents to an edge challenge that the image path reads as a corrupt image."
tags: [goisekai, image-cache, offline-first, prune, mangadex, challenge, user-agent, bug]
generated: { by: agent/cli, at: "2026-11-03T14:23:12Z" }
status: resolved
---

## Symptom

Manga `mangadex/a4a10d24-149f-4f7b-8fc1-2784d377fb08` showed no cover and no chapter pages although both had been cached before. The page renders from the DB (23 chapters, `cover_url` set), but every `<img src="/image?...">` came back 502, online or offline.

## How the offline-first image path works

`/view/manga/...` reads manga and chapters from the DB and emits `/image?pluginID&url=...`. `handleImage` calls `GetImage`, which probes `cachedImage` first (L1 in-memory map, then L2 `app_data/cache/images/<plugin>/library/<hash16>.<avif|webp|img|jxl>`; pages live under `<mangaID>/<chapterID>/`). The offline gate (`plugin is offline, cached image not available`) and the upstream fetch only run after both caches miss. The path is correct: a file on disk is served with no network. The failure was that the file was gone and could not be re-fetched.

## Cause 1: the cache is pruned oldest-write-first

`pruneImageCache` (`cmd/goisekai/prune.go`) runs at every startup and deletes the files with the oldest mtime until the total is at or below `max_cache_gb` (2.0 in `goisekai.ini`).

- Measured: the cache sits at exactly 2.000 GB (14,396 files); the oldest surviving file is 2026-10-19.
- This manga's cache dir was born 2026-10-02. Its chapter 1 page dir exists but is empty and its mtime is 2026-10-11 14:51, the moment its last file was removed. That fits oldest-first eviction.
- The cover is absent under all three URL variants (`.jpg`, `.256.jpg`, `.512.jpg`), hashed the way `diskCachePath` does (sha256 of `path?canonical-query`, first 8 bytes).

The policy is FIFO, not LRU: `cachedImage` reads a file without touching its mtime, and the prune knows nothing about library membership. Heavily read library content still ages out first. The old prune run is inferred from mtimes; the log only covers the current session.

## Cause 2: re-fetching is blocked by a MangaDex bot challenge

With the configured browser User-Agent, `uploads.mangadex.org` answers HTTP 302 `text/plain` to `https://mangadex.org/edge/challenge?src=...`. Followed, it returns 200 `text/html` (2,022 bytes). `fetchImageWithRetry` sees a 2xx, `validateImageFull` rejects the HTML, and the ladder logs `image corrupt` three times, then `invalid image data` and a 502.

| Request to the cover URL | Result |
|---|---|
| no UA, or `curl/8.0` | 200 `image/jpeg`, 60,912 bytes, decodes with Go stdlib |
| UA from `goisekai.ini` (Chrome/126), Chrome/131, Firefox/128 | 302 to the edge challenge |
| foreign Referer, no UA | 200, 59,480-byte placeholder (the known SHA256 case) |

39 `image corrupt` events in one session across many MangaDex covers that were not already in L2. Covers already on disk were unaffected (`L2 hit`).

## Why it compounds

Prune evicts, the re-fetch cannot refill, and the UI shows a bare 502 with no fallback (no cover blob in the DB). The image path does not treat "2xx but not an image" or a redirect to `/edge/challenge` as a `ChallengeError`, although `internal/hostnet` already has challenge and CDP machinery. Why `/image` does not use it was not checked.

## Fix options, ranked

1. Protect library manga from prune, or make eviction real LRU (touch mtime on an L2 hit).
2. Detect non-image or challenge responses on the image path and escalate through the hostnet ladder; a cheaper stopgap is a different UA for the MangaDex image hosts.
3. Raise `max_cache_gb` as a stopgap only.
4. Store a small cover thumbnail in the DB so the library renders without cache files.

Nothing was changed in the repo. Status: analysis only, to become an OpenSpec change.

# Related Concepts
- [Alt-cover picker cache mismatch (thumb vs full-size URL)](../plans/alt-cover-picker-cache-mismatch.md): Another case where the cover cache key and the URL actually requested diverge; same diskCachePath keying.
- [Hostnet CDP timeouts and page waits for SPA sites](../hostnet-cdp-timeout-waits.md): hostnet owns the challenge/CDP ladder the image path should escalate into.

## Outcome (2026-11-04, resolved in OpenSpec change `image-fetch-resilience`)

Both causes fixed in the same change.

**Finding added: browser UA triggers, empty UA gives 400.** Re-checked with `curl` against
`uploads.mangadex.org` and `api.mangadex.org` while planning: every browser-family
User-Agent (Chrome, Firefox, Safari) gets a 302 to `/edge/challenge` on both hosts;
a non-browser identity (`goIsekai/1.0 (personal reader)`, `curl/x`) gets 200. A *completely
empty* User-Agent is rejected with 400 — so the plugin-declared `user_agents` ladder
(`["goIsekai/1.0 (personal reader)", "default"]` in the MangaDex plugins) must never
fall through to empty; the literal `default` entry carries the host's configured
browser UA, which is blocked today by design, so it is a fallback rung only.

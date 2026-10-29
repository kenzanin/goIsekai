---
type: decision
title: Alt-cover picker cache mismatch (thumb vs full-size URL)
description: "Why picking an alt cover re-downloads although the grid showed it: the picker displays .256.jpg thumbs while setCover stores/downloads the full-size URL — different cache keys by design. Deferred fix: background-prefetch full-size candidates when the picker opens (option B)."
tags: [bridge, images, cover, mangadex, cache, deferred]
generated: { by: agent/mcp, at: "2026-10-29T09:02:26Z" }
status: stable
---

# Alt-cover picker vs setCover: two different URLs (2026-10-29)

## Symptom
Picking an alt cover on `/view/manga/mangafire/7j6v7` appeared to do nothing (502 after ~70s). The images rendered fine in the picker grid.

## Root cause
The picker grid and the pick action use **different URLs**, so the cache can never hit:
- Grid thumbs: `thumbURL()` in `internal/templates/views/detail.lua` rewrites MangaDex cover URLs to the `.256.jpg` variant (~60KB) — deliberate (grid is 128px, avoids MD@Home placeholder/rate-limit).
- Pick: `setCover` (`internal/bridge/enrichment_cover.go`) stores and downloads the **full-size** URL (0.5–5MB) — never fetched before, guaranteed cache miss (`diskCachePath` hashes the full URL by design).

## Decision
Leave as-is for now (user choice). Candidate fix when revisited: **option B** — when the picker opens, background-prefetch full-size candidates (PrioLow) so a pick usually hits L2 and completes instantly. Option A (store the 256px thumb as the cover) rejected: library covers render large.

## Related work this session (already shipped)
- `image_fetch.go` referer ladder: memo/caller ref leads, standard candidates follow; placeholder rejection advances the ladder (commit `4939d4f`).
- hostnet `doRequestProfile`: 3x retry on transient mid-body read errors via `isTransientReadErr`.
- Remaining failures on this host are environmental: ISP (ioh.co.id) intermittently TLS-shadows `uploads.mangadex.org` (x509 *.ioh.co.id served for it). curl succeeds 3/3; the Go client hits resets/shadowing intermittently. Fix belongs at network level (WARP/VPN), not app level.

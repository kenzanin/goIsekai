---
type: Concept
title: Cover refresh vs clear actions
description: What Refetch and Clear do on the manga detail cover menu, grounded in actions_covers.go and detail.lua.
tags:
  - goisekai
  - cover
  - finding
generated:
  by: process:omp-assistant
  at: "2026-10-11T02:45:00Z"
---

# Cover refresh vs clear actions

Grounded in `internal/httpserver/actions_covers.go` and
`internal/templates/views/detail.lua` (cover button row).

## Refetch (refresh) — `POST /action/refetch-cover/{pluginID}/{mangaID}`

- Handler: `handleRefetchCover` → `service.RefetchCover`.
- Effect: re-downloads the manga's **current main cover** from the source,
  busting the image caches.
- Toast: `Cover re-fetched`.
- Use when: main cover looks broken/stale/placeholder but the URL itself
  is right — this retries the download, it does NOT change which URL
  is picked.

## Clear — `POST /action/clear-covers/{pluginID}/{mangaID}`

- Handler: `handleClearCovers` → `service.ClearCovers`
  (drops stored alternative cover candidates, `alt_covers` table).
- Effect: deletes the **candidate list** shown in the Covers picker,
  NOT the main cover. Next `Covers` (fetch-covers) re-pulls candidates
  fresh from enrichment sources.
- Toast: `Cover candidates cleared` (matches `cover_clear_refresh.har`:
  `POST /action/clear-covers/1manga/yuru-fuwa-noka-no-moji-bake-skill`
  → 303 → `?toast=Cover+candidates+cleared`).
- Use when: picker shows stale/wrong candidates and you want a clean
  re-fetch.

## Sibling: Covers — `POST /action/fetch-covers/{pluginID}/{mangaID}`

- Handler: `handleFetchCovers` → `service.FetchCovers`.
- Effect: gathers alternative cover candidates from enrichment sources
  and stores them for the picker (`Covers` button, `data-open-covers`).
- Clear + Covers in sequence = full refresh of the candidate list.

## Evidence

- `cover_clear_refresh.har`: two `POST .../action/clear-covers/...`
  entries, both 303 to `?toast=Cover+candidates+cleared`, followed by
  detail-page GET and image fetch from `uploads.mangadex.org`.
- Main cover untouched by Clear; only the picker candidate store is dropped.

---
type: bug
title: Cover button on detail page silently triggers GetMangaDetail
description: "Why the cover picker button on the manga detail page re-invokes plugin detail fetch, and the code paths to check when it regresses."
tags: [goisekai, cover, enrichment, GetMangaDetail, detail-page, debug]
generated: { by: agent/mcp, at: "2026-10-31T04:57:53Z" }
status: stable
---

# Cover button on detail page silently triggers GetMangaDetail

## Symptom
User clicks the **cover** button (below the cover image on the manga detail page, next to **dim** and **fetch-cover**). The alt-cover picker modal opens correctly — but in the background a `GetMangaDetail` plugin call also fires. Repeats on every click.

## Detail-page cover menu (context for other models)
- **dim** — NSFW/vulgar-image sensor toggle
- **cover** — search alternative covers from enrichment sources (MangaDex, MangaUpdates)
- **fetch-cover** — same purpose as cover; user asked to remove it once, LLM refused because another feature depends on it; kept
- detail page also hosts the open-chapter button and other actions

## Root cause (fixed 2026-10-31)
The alt-cover fetch function (`FetchCovers` / `fetchEnrichmentKinds` in `internal/bridge/enrichment_cover.go`) contained a routine that fetched manga enrichment — and that enrichment path invoked the plugin's `GetMangaDetail`. So a UI button that "only fetches covers" silently did a full detail fetch. Fixed by removing/limiting the enrichment routine so the cover path only requests cover kinds.

## Debug pointers
- Debug log line to watch: `GetMangaDetail invoked` in `goisekai.log` (server must run with `-logLevel debug`)
- If the symptom returns: `rg GetMangaDetail internal/bridge internal/httpserver` and check whether any cover/alt-cover handler or enrichment helper calls back into the detail path
- Enrichment kinds filter: `fetchEnrichmentKinds` — the covers flow must pass a covers-only kind list

## Status
Fixed and verified by user 2026-10-31.

generated: { by: "reasonix-agent", at: "2026-10-31T12:00:00+07:00" }

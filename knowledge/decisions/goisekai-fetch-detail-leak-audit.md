---
type: decision
title: goIsekai fetch-detail leak audit (2026-10-30)
description: "Cover actions no longer trigger plugin detail or full enrichment: root cause chain, four fixes, verification, and tracing gotchas."
tags: [goisekai, enrichment, covers, htmx, debugging]
generated: { by: agent/mcp, at: "2026-10-30T13:24:20Z" }
status: stable
---

# Fetch-detail leak: resolved (commits e2cb381, 932c935, 3688e9d, 58bff09, e016ae6)

## User-visible bug
After fetch covers → pick cover → hide (dim), alt detail (categories/related/titles) appeared 'as if fetch detail ran', even after refresh.

## Debug tracing (commit 58bff09)
Debug logs added at: Manager.GetMangaDetail entry, all five cover action handlers, setCover before/after download, enrich fetch start (now logs kinds).

**Key evidence (goisekai.log, 2026-10-30 20:11-20:12):** `GetMangaDetail invoked` count = 0 for the whole session. The leak was NEVER a detail fetch.

## Actual root cause
`FetchCovers` called `FetchEnrichment(pluginID, mangaID, title, s.EnrichmentSources())` — a non-empty source list walks every provider and FetchFirst answers ALL kinds (titles, summaries, categories, related, authors, covers). So the Covers button silently stored full enrichment; a refresh then served it from the enrichment cache (`enrichment cache categories=7 related=7`).

## Fixes shipped
1. `e2cb381` — set-cover / refetch-cover / clear-covers render the cover-modal partial via shared `renderCoverModal` helper (X-Toast header) instead of toastRedirect; both JS submit handlers (~line 145 and ~line 863 in alpine-components.js) match `/fetch-covers|set-cover|clear-covers|refetch-cover/` and swap #cover-modal in place. set-cover also rewrites #cover-wrap img src and closes the picker; fetch/refetch reopen it.
2. `932c935` — toggle-cover-dim answered hxRedirect (full detail render → plugin invoke). Now returns JSON `{"dim":n}`; both JS handlers flip #cover-dim overlay + #cover-dim-btn label in place. Also fixed off-by-one: `action.split('/')[2]` → `[3]` for pluginID (was producing `/image?pluginID=set-cover&...`).
3. `3688e9d` — refetchCover called GetMangaDetail unconditionally (signed-CDN rotation guard). Now probes plugin detail ONLY when the cached cover URL download fails.
4. `e016ae6` — the actual alt-detail fill: `FetchCovers` now goes through `fetchEnrichmentKinds(..., []enrich.Kind{enrich.KindCovers})`; multi-source merge path filters kinds with slices.Contains; FetchEnrichment keeps nil = all kinds. enrich fetch start logs kinds.

## Gotchas
- GetMangaDetail response cached in plugin_cache TTL 24h; chapter list TTL 168h. Clear with sqlite3 app_data/goisekai.db "DELETE FROM plugin_cache WHERE plugin_id='<id>';".
- Internal logger: `logger.Debug(...)` from goisekai/internal/logger; Manager has no slog field.
- Serve with `-logLevel debug` to see the trace lines.

## Verification
All: just check 0 issues, just build ok, bridge+httpserver tests ok, live log shows no /view/manga render after cover actions. User confirmed fixed 2026-10-30.

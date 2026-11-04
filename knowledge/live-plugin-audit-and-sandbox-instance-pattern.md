---
type: decision
title: "Live plugin audit: sandbox instance pattern and per-plugin classification"
description: tools/audit-live-plugins.sh + sandbox -dataDir pattern that classifies all 47 live plugins per feature (search/detail/chapters-with-date/pages); mangahome genre parse fixed; mangahere/sanascans classified as content-level gaps; the rest deferred to follow-ups. Linked to the mangzio-genres and host-date concepts.
tags: [goisekai, plugins, audit, sandbox-instance, lua-patterns, remediation]
generated: { by: agent/mcp, at: "2026-11-04T13:04:02Z" }
status: stable
---

# Live plugin audit (2026-11-04)

`tools/audit-live-plugins.sh` probes every active plugin through the live host's sandbox read-only endpoints: `/api/sandbox/plugins/{id}/search|detail|chapters|pages` with `q=hero` falling back to `q=manga` on an empty first probe, so a q-term that misses a catalog no longer hides a working plugin. Per-plugin per-feature classification: WORKING (data present), MISSING (HTTP 200, data absent), BROKEN (HTTP error — 403/500 labeled), with a `chapters_with_date` count column.

## Sandbox-instance pattern

`./goisekai -dataDir app_data_playground -port 8081` runs an isolated instance (own DB, plugins, cache, PID) next to production on 8080. Both read the same plugin set only if the sandbox's `app_data_playground/plugins/` is synced from `examples/plugins/` — edits land in `examples/` (git-tracked), then `cp examples/plugins/lua/<id>/*.lua app_data_playground/plugins/<id>/` and restart the instance for lua; wasm/native need a rebuild. The audit writes `live-plugin-audit.json`/`.md` at the repo root.

## Baseline (47 plugins)

- Most plugins fully WORKING (search/detail/pages, chapters with dates).
- **mangahome detail genres — fixed:** the site changed genre chips from `href="/directory/{slug}"` to `href="/{slug}"`, and the Lua pattern also used `\s` (invalid in Lua patterns — must be `%s`) and `\(s\)` (must be `%(`). Correct pattern: `"Genre%(s%):</span>%s*.-</p>` cell extraction. Re-audit now classifies mangahome detail WORKING (`genres: ["Action","Mature","Shounen"]`).
- **mangahere detail MISSING on one title** — the sampled manga's `Genre(s)` cell is genuinely empty on the site; not a parser bug. Correctly a content observation, not a fix.
- **sanascans detail MISSING** — intermittent site unreachability confirmed by direct `curl` (200) vs host 500. Not fixable from plugin source.
- **Deferred follow-ups (user decision 2026-11-04):** `fenyk`, `mangade` (search returns an empty list for both q terms — needs source investigation), `mangataro`/`zeroscann` (lua context-deadline on search), `onisaga` (hostnet anti-bot challenge), `komix_to`/`lhtranslation`/`reimanga_net` (site unreachable/detail 500 — wasm binary rebuild path, no readable source inside `app_data/`). All have readable `examples/plugins/*` sources per user instruction; edits land there, never directly in `app_data/` (untracked).

## Audit-script root causes fixed along the way

The initial 10-row broken baseline came from a script version that probed the bridge `/api/manga` and `/api/search` endpoints — the bridge detail omits `genres`, so the whole wall read MISSING — and whose node probe validation crashed on truncated bodies. Root cause for the bogus MISSING-detail classification: endpoint-shape mismatch; the fixed script probes the sandbox routes and JSON-validates with a try/catch fallback.

Lua-pattern lesson for the next parser fix: use `%s`-class escapes and `%(...%)` literal-paren escaping, verify with a tiny offline `lua` script before shipping (`lua $TMPDIR/mh5.lua`), and confirm against the real upstream HTML the live page currently serves.

# Related Concepts
- [Mangzio genres: balanced RSC parsing and getGenres export](mangzio-genres-balanced-rsc-and-getgenres-export.md)
- [Host date source of truth](host-date-source-of-truth.md)

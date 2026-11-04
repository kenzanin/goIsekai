---
type: decision
title: "Live plugin audit follow-ups: remaining broken/missing plugins and their first probes"
description: "List of remaining work after live-plugin-feature-audit completed: empty-search lua plugins (fenyk, mangade), mid-request failures (zeroscann, mangataro), hostnet cloudflare challenges (onisaga), wasm/native site rebuilds (komix_to, lhtranslation, reimanga_net), audit-script debt, and OKF orphan concepts. Each with a first probe to run."
tags: [goisekai, plugins, follow-up, audit, remediation]
generated: { by: agent/mcp, at: "2026-11-04T13:15:21Z" }
status: draft
---

## What remains after live-plugin-feature-audit (2026-11-04)

Recorded per user request after change completion, so the deferred scope is not lost. Every item has a known first probe; the baseline to beat is `live-plugin-audit.json` (47 plugins, playground instance 8081).

### 1. Lua plugins with EMPTY search (highest visibility)
- `fenyk` — `search q=hero` and `q=manga` both return `[]`. First probe: run the query `curl "https://fenyk.org/api/v1/search?limit=20&page_no=1&q=hero"` bare, then with the plugin's goisekai UA — the API answers a bare call 200, so the difference is either the plugin's `filter=popular` tag-mapping default or an account/auth header the browser sends and the plugin omits.
- `mangade` — same shape (search empty both q terms). First probe: compare the site's actual search XHR method (GET vs POST) against `examples/plugins/lua/mangade/main.lua`'s assumed one.

### 2. Lua plugins failing MID-REQUEST (search errors)
- `zeroscann` — search dies with `[string "main.lua"]:22: context deadline exceeded`. Line 22 hosts the initial `host.http.get`. First probe: `lua -e '...'` a local shell against `https://zeroscann.*/search?q=hero` to see if the site itself is slow (bump TIMEOUT in the script) or if the request URL is wrong (404 body loops into an error, not a slow site).
- `mangataro` — search breaks wasm dead with `module closed with context deadline exceeded`; the site `mangataro.org` answers a bare `curl` 200. First probe: replay the request with the exact goisekai UA header the plugin sends, look for a bot-block the host turns into a timeout.

### 3. Hostnet anti-bot challenges
- `onisaga` — search dead with `hostnet: challenge at https://onisaga.com/search/manga`, meaning the goisekai UA hit Cloudflare and the plugin's declared UA ladder never got to walk (only one UA entry is seeded). First probe: check if `plugin.json` carries `user_agents: []` so the ladder only has goisekai itself, then either add browser UAs there or accept a seed-cookie screenshot flow.

### 4. Site-level unreachability (wasm/native, no source fix inside the binary)
- `komix_to` — search WORKING, detail dead with `module closed with context deadline exceeded` on **detail** specifically. Self-pages are fine; only `api.mangadex-like` detail path dies. First probe: `curl` the same detail URL the wasm binary builds; if HTTP 200 comes back with data, the binary needs rebuilding from its `examples/plugins/native/komix_to/` Go source (update any stale path/regex and run its Makefile).
- `lhtranslation` — search WORKING (but the first result has a blank `cover_url`/title of `feed` — likely a list-side regex misses the real anchor), detail dead with `plugin lhtranslation is unreachable and manga feed has no cached copy`. First probe: fix the strip after search, then track down which host helper died.
- `reimanga_net` — search WORKING, detail dead the same way. Same first probe as komix_to (native source lives at `examples/plugins/native/reimanga_net/`) — rebuild the binary from its Makefile.

### 5. Audit-script debt (not blocking, note for the next run)
- `probe_search` sets a global `Q`, so a plugin that flips `Q` to `manga` silently leaves it for every later plugin — the `hero`-first rule degrades for later rows. Fix: scope `Q` locally inside `probe_search`.
- `probe_search` `chapters_with_date` counts only catch dates a `chapters` endpoint returns; if a plugin has no such endpoint (exclusive to a few), the column shows 0 without meaning "broken".

### 6. Change-archive follow-ups
- Some earlier-archive orphan concepts flagged by `okf_validate`: `plans/har-plugin-candidates-sanascans-first`, `live-plugin-feature-audit-sandbox-remediation`, `bugs/cover-button-fetch-detail`, `decisions/goisekai-fetch-detail-leak-audit`, `decisions/max-cache-gb-settings-editable-and-hot-prune`, `plugin-authoring-guide` — are not linked to any parent. Staple each into the right `index.md` or delete if actually stale.

All observed via `live-plugin-audit.json`, `$TMPDIR/audit_*.search/detail` probe bodies, and `okf_validate` output; wire up in the next OpenSpec change proposal using `/opsx:apply`.

# Related Concepts
- [Live plugin audit: sandbox instance pattern and per-plugin classification](live-plugin-audit-and-sandbox-instance-pattern.md): Enumerates what remains broken/missing after the audit change completed

# Proposal

## Why

The codebase has 47 installed source plugins spanning five runtimes (lua, js, yaegi, native-go, wasm), plus enrichment scripts under `app_data/info/`. Plugin fixes so far were reactive: a user hits a broken page (empty genres, missing dates, failed search) and the plugin gets patched one at a time. There has never been a systematic live audit that checks every installed plugin end-to-end through the running host, so we do not know which plugins are silently broken (search dead, no dates, no genres, page-list failures) and which are fine.

Established in previous changes (host-date-source-of-truth, mangzio genre work): the host only accepts correctly-shaped data from plugins, optional exports (GetGenres) have sticky caches that record their absence, and JS plugin errors only surface live (ReferenceError → HTTP 500 that `node --check` misses). An audit harness that exercises the live ABI per plugin turns "is it working?" from guesswork into a verified table.

## What Changes

- Add an audit capability that probes every installed (active) live plugin through the host API and reports per-plugin, per-feature status: search, detail, chapter list (+ released_at presence), page list, genres catalog.
- Produce a classified result table: feature-complete plugins vs plugins with a specific missing feature (e.g. no released_at, no genres, search error, page_list empty).
- For each broken plugin found, apply the minimal source fix following the established patterns (balanced object parsing for RSC payloads, `date_to_iso` for dates, getGenres scrape from catalog, per-page headers for gated CDNs), then clear the corresponding sticky caches and re-verify live.
- No ABI or host behavior changes: audit output is a report on existing behavior; plugin fixes stay inside `examples/plugins/` + `app_data/plugins/` (synced) and `app_data/info/` untouched unless a broken enrichment is found.
- Every audited plugin and every fix is recorded as a node in the OKF knowledge bundle so the next plugin author inherits the findings.

## Capabilities

### New Capabilities
- `live-plugin-audit`: an audit procedure (script + documented process) that exercises all live plugins through the host HTTP API and classifies per-feature completeness.

### Modified Capabilities
<!-- None: host behavior, ABI shapes, and existing spec requirements are unchanged. If the audit surfaces a plugin bug that requires touching host code, that would be a separate propose. -->

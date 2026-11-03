---
type: decision
title: "live-plugin-feature-audit: sandbox audit + comix/sanascans remediation"
description: "Findings and decisions from the live-plugin-feature-audit OpenSpec change: -dataDir sandbox instance, tools/audit-live-plugins.sh, comix pagination fix (40->2, search ~80s->~10s), sanascans slug decode fix (url_decode after html_decode), upstream outages classified not-fixable, and the gotcha that chapter-date fixes require per-site markup probing."
tags: [goisekai, openspec, live-plugin-audit, comix, sanascans, dataDir, sandbox, audit, released_at, getGenres, deploy]
generated: { by: agent/mcp, at: "2026-11-03T01:14:24Z" }
status: stable
---

## Findings & decisions

1. -dataDir flag + GOISEKAI_DATA env in cmd/goisekai/main.go; sandbox: ./goisekai -dataDir app_data_playground -port 8081 (own DB/plugins/PID).
2. tools/audit-live-plugins.sh: probes search/detail/chapters/pages + sandbox raw probes, writes live-plugin-audit.{json,md}.
3. comix: search loop 40 -> 2 pages (pages 3+ 404 upstream; loop blew the 30s budget). Search ~10s/30 results after fix.
4. sanascans: slug = host.text.url_decode(html_decode(slug)) (single pass; an earlier DOUBLE url_decode was wrong because '&' is HTML-entity-escaped, not %-encoded). detail 404 -> 200.
5. Sync triangle: examples/plugins/ -> app_data/plugins/ (prod) AND app_data_playground/plugins/ (sandbox), then POST /api/sandbox/plugins/<id>/reload (WITH /api prefix). Promote only after sandbox re-probe passes.
6. Not fixable in source: fenyk (530), mangade (000 unreachable), mangadex/dynastyscan (0 matches for q=hero), onisaga (403 challenge).
7. Gotcha: chapter-date gaps are per-site work - mangahome has site_date_to_iso(date_s) at main.lua:232 yet emitted 0 released_at live; grep-derived date lists overstate fixability, probe markup first.

## Verification
just check (0 issues) + just build OK; prod restart -> http://localhost:8080/ 200. comix/sanascans verified per-plugin via sandbox reload + re-probe.

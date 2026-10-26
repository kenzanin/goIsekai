---
type: Concept
title: Session dailies 2026-08-29 to 2026-09-30
description: "Roll-up of 8 daily scratchpads - tls-client migration, Human Verify/WebP ship, image burst pacing, tailwind path bug, wasm host helpers, decimal-chapter audit, FanFox."
tags: [goisekai, daily, session-log]
generated: { by: agent/mcp, at: "2026-10-26T16:00:07Z" }
status: stable
---

# Session dailies 2026-08-29 to 2026-09-30

Consolidated roll-up of 8 EchoesVault daily scratchpads.

## 2026-08-29 - hostnet proxy migrated to tls-client

- internal/hostnet/proxy.go moved from net/http Client + cookiejar to tls-client: lazily-created map[string]tls_client.HttpClient (one per plugin id, sync.Mutex guarded), each with WithTimeoutSeconds(30), WithClientProfile(Chrome_146), WithCookieJar.
- buildHeaders assembles the COMPLETE header set (defaults + sorted overrides, empty-Referer skipped).
- Branch omos/tlsclient, commit 9bb18af. CGO_ENABLED=0 OK.

## 2026-08-31 - Human Verify v1 + WebP cache + image burst pacing

- Human Verify v1: cookie-paste verify panel, challenge detection (403/53), amber banners, disk cache converts jpeg/png to webp q85.
- Reader Retry re-fetch preserves current page.
- Image burst pacing ROOT CAUSE of recurring 404: MD@Home 404s concurrent bursts (not URL expiry). Fix in bridge GetImage: 1-slot semaphore + per-host pacing >=1.1s + 3x quadratic-backoff retry. Live-verified: 4 parallel fresh fetches -> all 200, zero retries.
- Tailwind path bug: was writing to frontend/static/lib/ (never served). Fixed to frontend/lib/tailwind.css. Need make br.

## 2026-09-07 - library sidebar

- library.jet: stats cards moved into left sidebar aside.lg\:w-56.

## 2026-09-10 - cover thumbnail bugs

- Search page missing covers: search.lua computed coverHTML but never concatenated into result card. Fix: insert .. coverHTML.
- Updates page: tailwind.css overwritten with 37B source directive, make br compressed it. Fix: separate @tailwind input file.

## 2026-09-11 - plugin helper duplication audit

- 12 plugins (5 JS, 7 Lua), 30 source files. enrich.js byte-identical in js 3; normalizeStatus 8 diverging copies. Host natives exist unused.

## 2026-09-23 - wasm host helpers + plugin ports

- Wasm host helpers: env.host_call(ptr,len)->i64 import; dispatches JSON envelope. Docs: docs/plugin-wasm-helpers.md. Need docs/plugin-wasm-helpers.md (in okf-mcp/docs/).
- Wasm ports: mangaka, madaradex, ksgroupscans.
- mangaka: api.mangak.io JSON (gated by Accept + Referer). Gotcha: httpBody(url) broken; correct httpBody("http.get_body", u, headers).
- mangakakalot search fix: ww2.mangakakalots.com dead; switched to www.mangakakalove.com.

## 2026-09-26 - decimal-chapter bug class + audits

- mangabuddy page-list regex: \d+ -> [\d.]+ (live 0 -> 13 pages for chapter 7.1).
- valirscans same: [\d.]+.
- Cross-plugin audit: only mangabuddy + valirscans affected.

## 2026-09-30 - FanFox

- Implemented FanFox Lua plugin, ABI contract_version 1.

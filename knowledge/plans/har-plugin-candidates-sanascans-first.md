---
type: decision
title: "HAR plugin candidates: sanascans first"
description: "HAR-vs-plugin audit 2026-10-29: 7 HARs lack plugins; sanascans (SSR TanStack, ISO dates, no auth) chosen as easiest next plugin; full plan with search/detail/chapter strategy and fallbacks recorded."
tags: [plugins, har, sanascans, mangadot, plan]
generated: { by: agent/mcp, at: "2026-10-28T19:03:53Z" }
status: stable
---

# HAR sweep: plugin candidates (2026-10-29)

## HARs without a plugin in app_data
| HAR | Site type | Feasibility |
|---|---|---|
| sanascans.har | TanStack Start, SSR + `_serverFn` RPC | ⭐ easiest — chosen |
| mangadot.har | public JSON API `/api/discovery/...` | ⭐ next |
| mangaball.har | Next.js `__next_f` stream | medium |
| xbato.har | API needs auth (`/api/auth/me`) | medium-hard |
| readmanga / mkissa / todaymanga | no chapter/date evidence in HAR | hard — HAR lacks data |

(comix, mangahere, mangapill, mangaread HARs already have plugins. library_filter.har is a localhost HAR — not a candidate.)

## Plan: sanascans plugin
- Base: `https://sanascans.com`
- Evidence from HAR: `/comics` SSR page 655KB with ISO dates; `/series/<slug>` SSR 180KB with ISO dates; `_serverFn/<hash>` RPC endpoints return plaintext JSON with ISO dates, no auth, no encryption.
- search_manga: regex SSR `/comics` HTML → title/slug/cover (cover CDN storage.sanascans.com, no hotlink guard observed).
- get_chapter_list: regex `/series/<slug>` SSR HTML for chapter rows + ISO dates; fallback `_serverFn` hash from HAR if HTML lacks rows. Dates already ISO → `host.text.date_to_iso` passes through; set `ch.released_at`.
- get_manga_detail: parse `/series/<slug>` SSR (title, cover, synopsis, genres).
- Mirrors proven mangahere pattern (SSR regex + date_to_iso funnel). No CDP expected.

## Decision
- Build sanascans first, mangadot second if wanted. xbato/mangaball deferred (auth/stream parsing).

## Session context
- Comix work this session (chapter released_at 9/9 via DOM evaluate + relative_to_iso; search pagination 266; detail via browser.fetch to dodge ISP cert shadow *.ioh.co.id; jobs.go 120s; plugin.json timeout 180) is UNCOMMITTED, pending commit approval.
- JSON.parse-sniffing of comix signed API proven non-viable (bundler captures JSON.parse before init scripts; 0 hook invocations).

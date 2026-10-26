---
type: reference
title: goIsekai agent handover
description: "Agent handover doc — project, commands, runtime state, shipped work, critical rules."
tags: [goisekai, handover, agent, reference]
generated: { by: agent/mcp, at: "2026-10-26T20:14:19Z" }
status: stable
---

# goIsekai agent handover — 2026-10-10

Switching from `pi` harness to `omp`. Everything an agent needs to resume without prior session memory.

## 0. Project in one paragraph

goIsekai: manga library manager + reader, embedded HTTP server + browser UI. Go 1.27, module `goisekai`. Plugins (Lua/lunar, JS/goja, Go .so, Yaegi, wasm) fetch manga from external sites. Pure-Go SQLite (`CGO_ENABLED=0`), go-jet DSL queries, Lua HTML templates, Tailwind + Alpine frontend. Read `AGENTS.md` first — it has the full architecture map and command table.

## 1. Commands

| Task | Command |
|---|---|
| Build (Go only) | `just build` |
| Format + lint + tests (gate) | `just check` (0 issues = green) |
| Full tests | `just test` |
| Lua template lint | `just lint-lua` |
| Frontend format | `just fmt-web` (biome) |
| Regenerate brotli after JS/CSS edit | `just br` — **mandatory** |
| Run server | `./goisekai` or via tray `\./goisekai-tray` |

`just build` does **not** build `goisekai-tray` — build manually: `go build -o goisekai-tray ./cmd/goisekai-tray`.

## 2. Runtime state (2026-10-10 10:10 +0700)

- Tray PID 430999 → server PID 431009, port 8080, started 10:10.
- Binary built ≈ HEAD 635ed48. Rebuild + restart tray after Go changes.
- **onisaga cookies DEAD** (probe 09:47 → 403, age 230 min). Re-paste via the wizard before any live onisaga test.
- `cdp_engine = obscura` (usually dead; 120s timeout vs Cloudflare).
- `api_key` empty in ini.

## 3. Shipped this session

635ed48 — pluginmanager: propagate invoke ctx into JS + wasm proxy calls. 202a021 — docs: add handover. 5855d16 — css: wizard modal utility classes. 9822609 — manual_cookies=true. 555e561 — challenge bounded by invoke ctx (15s). dd79b90 — save never blocks on plugin mutex.

## 6. Critical rules

- NEVER CGO_ENABLED=1 for builds (race only).
- State-changing POSTs on `/action` (CSRF-protected).
- User-facing error text is host-owned; plugins return error-code strings only.
- URL-encode plugin IDs in template paths.
- FTS: every mutation SyncFTS.
- Plugin invocations run on context.Background()+15s.
- Lua empty-table: {}→[], request headers stay {}.

---
type: Concept
title: Wasm plugin local helpers decision
description: "Why wasm plugins keep local helpers instead of per-row hostCall — ABI/JSON roundtrip cost, status-map plugin ownership, coregex/go-json binary-size tradeoff."
tags: [goisekai, wasm, plugin, abi, decision]
generated: { by: agent/mcp, at: "2026-10-26T15:26:23Z" }
status: stable
---

# Wasm plugin local helpers decision

(source: EchoesVault page `wasm-plugin-local-helpers-decision.md`; audit
2026-09-26)

## Context

Audit asked why wasm plugins duplicate host capabilities (`chapterNum`,
`normalizeStatus`) instead of calling `host.text.*` helpers like Lua/JS
plugins do.

## Decision

**Keep local helpers in wasm plugins for hot paths. Do not force parity with
Lua/JS call patterns.**

### Reasons

1. **ABI cost**: every `hostCall` from wasm is an IPC hop + JSON
   marshal/unmarshal through `internal/pluginmanager/wasm_host_helpers.go`. Lua/JS host functions are cheap in-VM calls; wasm host calls are not. Parsing a 200-chapter list with per-row `text.chapter_num` + `text.url_encode` ≈ 400 JSON roundtrips vs 0 with local `regexp` + `strconv`.
2. **Status mapping is plugin data**: `normalizeStatus` maps site-specific
   labels ("Ongoing", "Publishing", "Airing") to canonical status. The
   mapping belongs in the plugin; the host only provides
   `text.normalize_status(map, raw)`. Site label changes must not require a
   host binary rebuild.
3. **Binary size**: host uses `coregx/coregex` (O(n) anti-ReDoS) as a
   security boundary for *untrusted* plugin patterns. Wasm plugins only
   compile their own static patterns with stdlib `regexp` (RE2-style,
   ReDoS-safe). Pulling coregex into wasm would add hundreds of KB per ~5MB
   `main.wasm` with no perf gain. Same for `goccy/go-json` vs stdlib
   `encoding/json` (one decode per invoke).

### Pattern (current state)

- Lua/JS plugins: full `host.text.*` / `host.regex.*` / `host.json.*` usage —
  preferred for those runtimes.
- Wasm plugins: local helpers for hot loops; `hostCall` for heavy/singleton
  work (HTTP, crypto, VRF).
- wasm `go.mod`: only `require goisekai v0.0.0` +
  `replace goisekai => ../../../..`. No third-party deps.

### When to revisit

- If a wasm plugin processes >1k rows per invoke and profiling shows helper
  cost, batch via a single hostCall instead of switching to per-row calls.
- If plugin authors want coregex semantics, they may `go get` it in the
  plugin module — the accepted size tradeoff is theirs.

## Related

- `internal/pluginmanager/wasm_host_helpers.go` — `text.*` dispatch incl.
  `text.chapter_num`, `text.normalize_status`
- `internal/pluginutil/` — shared helper implementations (ChapterNum,
  NormalizeStatus, URLEncode…)
- `examples/plugins/wasm/*/main.go` — local `chapterNum`,
  `normalizeStatus` implementations

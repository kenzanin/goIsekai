# Design

## Context

47 source plugins are installed live (spread across lua/JS/yaegi/native-go/wasm runtimes) plus enrichment scripts under `app_data/info/`. Past fixes were one-off, driven by user-visible breakage. The host exposes a uniform HTTP API surface each plugin reaches through bridge calls, which makes a black-box audit cheap: one invocation per plugin per feature, no plugin-code execution needed by the auditor besides what the host already runs.

Established facts the audit design leans on:
- Search genre per item comes from the plugin's own payload (JS chunk balanced-object work, mangzio ce39aab).
- Chapter released_at comes from plugin chapters; the host routes it by JSON type (epoch seconds or RFC3339 via types.Chapter.UnmarshalJSON).
- The genre filter is served from the bridge's plugins.genres cache with sticky "absent" semantics ("[]" never re-invoke, mangzio e6e0b3f).
- JS plugin ReferenceError surfaces as HTTP 500 only live (node --check misses it).

## Goals / Non-Goals

Goals:
- A repeatable audit run that classifies per-plugin/per-feature health through the live host API.
- Output that is directly actionable: which feature, which plugin, which pattern to apply.
- Every finding recorded in the OKF knowledge bundle (this project's convention for agent memory).

Non-Goals:
- Changing host code, ABI, templates, or plugin.json schema.
- Rebuilding wasm plugins during the audit itself (out of scope; separate build task if source fix lands).
- Parallel scraping stress on sources (sequential probes, short timeouts).

## Approach

A single audit script (shell, driven by curl against the running host) walks the plugin list and, per plugin, issues four probes against existing endpoints:

1. `GET /api/search?pluginID=<id>&q=hero` (safe common term) → HTTP status + count of results + whether the first item carries a genres array.
2. `GET /api/manga/<pluginID>/<first-result-id>` → HTTP status + genres/status presence.
3. Chapter list is inside the detail response → count chapters and whether any carries released_at.
4. `GET` reader-data or page-list fetch for the first chapter id → page count > 0.

All outputs collected into a single markdown/JSON table. Feature classified:
- WORKING - expected data present.
- MISSING - HTTP 200 but data absent (e.g. no genres in items, no released_at).
- BROKEN - HTTP error (4xx/5xx) or empty payload from a healthy host.

Alternative considered and rejected: loading plugins in a test harness executing exports directly in Go (go test-style). Rejected because it bypasses bridge caching, challenge flow, and caches (the very things that have bitten us), and it would not test the same surface users hit.

## Implementation Notes

- Sequential probes with ≤15s curl timeouts; plugin count ~47 → wall-clock bounded by slowest per-plugin latency, acceptable offline.
- Any remediation (patching plugin source in examples/ + app_data/, cache NULL reset, `just` build/restart, re-probe) follows the established deploy gate from the deploy-gate memory (just check && just build, restart, verify root 200).
- OKF note goes in `knowledge/` via okf_create; the okf toolchain auto-validates links/orphans.
- No new host endpoint added; the audit script is repo-local tooling, not user-facing surface.

## Risks

- Slow or rate-limited sources stretch wall-clock; mitigated by per-request timeouts and sequential execution.
- Challenge-gated plugins can report BROKEN wrongly when the source simply requires manual cookie solving; the report distinguishes 403 ChallengeError explicitly so these are labeled, not confused with code bugs.
- Category drift: plugin source files may change between audit and remediation; the audit records the plugin id + observed data, not a commit hash, so the re-probe after a fix is the source of truth.

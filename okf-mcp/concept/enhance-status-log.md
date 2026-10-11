---
type: Concept
title: Enhance status startup log
description: Startup log of effective image-enhance status per plugin — formatEnhanceStatus/LogEnhanceStatus wiring, log format examples, edge cases.
tags:
  - goisekai
  - enhance
  - logging
  - finding
generated:
  by: process:omp-assistant
  at: "2026-10-11T02:58:59Z"
---

# Enhance status startup log

Added a startup log showing the effective image-enhance status for all
plugins (source: EchoesVault page `enhance-status-log.md`).

## Changes

- `internal/bridge/enhance.go`: `formatEnhanceStatus()` method on `enhanceConfig`.
- `internal/bridge/service.go`: `LogEnhanceStatus()` method on `AppService`.
- `cmd/goisekai/main.go`: calls log-enhance-status at startup.
- `internal/bridge/enhance_test.go`: table-driven tests.

## Log format examples

- With plugins & overrides: `enhance status: default=auto | off: mangadex | on: kaliscan, bato1`
- No overrides: `enhance status: default=auto | on: kaliscan, bato1 (no overrides)`
- Empty plugin list: `enhance status: default=off (no plugins)`
- With unused override: `enhance status: default=auto | off: mangadex | unused overrides: typo`

## Edge cases handled

1. Empty plugin list → logs `(no plugins)`.
2. Unused overrides → logs `unused overrides: <plugin>`.
3. Info-only scripts (enrichment) are excluded from the plugin list.

## Gates at time of writing

gofmt PASS, build PASS, tests PASS, lint PASS (only pre-existing issue).

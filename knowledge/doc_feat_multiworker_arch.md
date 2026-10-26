---
type: reference
title: Multi-Worker Architecture
description: "Multi-worker architecture proposal: separate lanes for interactive vs fetch vs image vs maintenance."
tags: [goisekai, workers, architecture, plan]
generated: { by: agent/mcp, at: "2026-10-26T20:17:44Z" }
status: draft
---

# goIsekai multi-worker architecture

**Status:** proposal / analysis only (no code landed).
**Scope:** separate worker lanes for UI/interactive handling and manga/cover fetching.

## Goals

1. UI interactive at all times — user requests never wait behind bulk fetches.
2. Move manga metadata fetching and cover/page fetching onto dedicated workers.
3. Replace goroutine-per-request blocking with bounded queues + observable job state.

Non-goals (YAGNI): distributed queues, persistent job stores, generic task framework.

## Current state

Everything runs on HTTP request goroutine or one maintenance goroutine. Export/sync block requests for tens of seconds; paceImage 1100ms sleeps paid by request goroutines. Plugin invocations run on context.Background()+15s.

## Proposed topology

Four lanes:
- InteractiveWorker: Search/GetMangaDetail/GetChapterList/GetPageList (4 workers, queue 32, highest).
- FetchWorker: SyncLibrary/SyncManga/migration/enrichment/cover download (2 workers, queue 128, per-plugin fairness).
- ImageWorker: cover/page fetches (8 workers; high 64 / low 256). Keeps hostAcquire + paceImage as transport gate.
- MaintenanceWorker: prune/FTS-backup/image-prune (1 worker, ticker).

Rules: pools independent; handlers enqueue+await future; backpressure; dedupe at enqueue.

## Config (proposed INI)

```ini
[workers]
interactive = 4
fetch = 2
image = 8
queue_interactive = 32
queue_fetch = 128
queue_image = 256
```

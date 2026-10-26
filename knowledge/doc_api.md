---
type: reference
title: goIsekai JSON API
description: "Machine-facing JSON API for custom UIs, scripts, and wrappers."
tags: [goisekai, api, json, reference]
generated: { by: agent/mcp, at: "2026-10-26T20:13:52Z" }
status: stable
---

# goIsekai JSON API

Machine-facing API for custom UIs, scripts, and wrappers. Same bridge service layer as the web UI — same data, consistent shapes.

## Base URL

http://127.0.0.1:8080  — all endpoints under /api/.

## Authentication

Optional. Set via `-apiKey` flag or `api_key` in goisekai.ini. When set, every /api/ request needs `X-API-Key: <key>`. Missing/bad key → 401 `{"error":"unauthorized"}`. When no key configured, all /api/ routes served without check. `GET /image` is exempt (browsers can't send headers).

## Errors

All errors: `{"error":"<message>"}`. Status codes: 400, 401, 404, 502. Field naming: new endpoints use snake_case. `/api/reader-data` keeps camelCase (documented, not migrated).

## Endpoints

- `GET /api/library` — list in-library manga with progress stats.
- `GET /api/search?pluginID=<id>&q=<q>&page=<n>` — search catalog. Returns `{results,has_next}`.
- `GET /api/manga/{pluginID}/{mangaID}` — detail with chapter list.
- `GET /api/history` — reading history.
- `GET /api/reader-data/{pluginID}/{mangaID}/{chapterID}` — page list (camelCase, existing).
- `POST /api/library/toggle/{pluginID}/{mangaID}` — add/remove. Returns `{in_library}`.
- `POST /api/chapters/read/{pluginID}/{mangaID}/{chapterID}` — mark read/unread. Returns `{is_read}`.
- `POST /api/progress/{pluginID}/{mangaID}/{chapterID}` — set page. Body `{"page":15}`. Returns `{page,total_pages}`.

## Curl

```
curl -s http://127.0.0.1:8080/api/library | jq
curl -s -H 'X-API-Key: key' http://127.0.0.1:8080/api/search?pluginID=mangzio&q=isekai | jq
curl -s -X POST -H 'X-API-Key: key' http://127.0.0.1:8080/api/library/toggle/mangzio/solo-leveling | jq
```

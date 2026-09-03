# goIsekai JSON API

Machine-facing API for custom UIs, scripts, and wrappers. Same bridge service
layer as the web UI — same data, consistent shapes.

## Base URL

The API lives on the same host and port as the web UI:

```
http://127.0.0.1:8080
```

All endpoints are under `/api/`.

## Authentication

Authentication is **optional**. Set an API key via the `-apiKey` CLI flag or
`api_key` in `goisekai.ini`. When set, every `/api/` request must include the
header:

```
X-API-Key: <your-key>
```

Requests without the correct key (or with no header) return `401`:

```json
{"error":"unauthorized"}
```

When **no key is configured**, all `/api/` routes are served without any check.

> **Warning:** If you bind the server to a non-loopback address (e.g. `0.0.0.0`)
> without setting an API key, the entire API — including the plugin sandbox
> hot-load endpoints — is exposed to your network. Always set `-apiKey` when
> binding externally.

`GET /image` is exempt from the key check so `<img>` tags work in browsers
without custom headers.

## Errors

Every error response is a JSON object with an `error` field:

```json
{"error":"<message>"}
```

Status codes:

| Code | Meaning |
|------|---------|
| `400` | Missing or invalid parameters |
| `401` | Bad or missing API key |
| `404` | Unknown resource |
| `502` | Upstream plugin call failed |

Success payloads are bare JSON (no wrapper).

## Field Naming

New endpoints use `snake_case`. The pre-existing `/api/reader-data` endpoint
keeps its original `camelCase` keys for backward compatibility. This
inconsistency is documented, not migrated.

## Endpoints

### `GET /api/library`

List in-library manga with progress stats. Returns an array, most recently
updated first.

```json
[
  {
    "manga_id": "manga-123",
    "plugin_id": "mangzio",
    "source_manga_id": "solo-leveling",
    "title": "Solo Leveling",
    "cover_url": "https://example.com/covers/solo-leveling.jpg",
    "total_chapters": 180,
    "read_chapters": 42,
    "has_new": false
  }
]
```

### `GET /api/search`

Search a plugin's catalog.

| Param | Required | Description |
|-------|----------|-------------|
| `pluginID` | yes | Plugin to search |
| `q` | yes | Search query |
| `page` | no | Page number (default `1`) |

```json
{
  "results": [
    {
      "id": "solo-leveling",
      "title": "Solo Leveling",
      "cover_url": "https://example.com/covers/solo-leveling.jpg"
    },
    {
      "id": "solo-leveling-arise",
      "title": "Solo Leveling: Arise",
      "cover_url": "https://example.com/covers/arise.jpg"
    }
  ],
  "has_next": true
}
```

### `GET /api/manga/{pluginID}/{mangaID}`

Manga detail with chapter list (newest-first), per-chapter progress, and a
continue point.

```json
{
  "id": "solo-leveling",
  "title": "Solo Leveling",
  "author": "Dubu (Redice Studio)",
  "cover_url": "https://example.com/covers/solo-leveling.jpg",
  "status": "ongoing",
  "description": "In a world where hunters must battle deadly monsters...",
  "genres": ["action", "adventure", "fantasy"],
  "chapters": [
    {
      "id": "chapter-180",
      "chapter_num": 180,
      "title": "Chapter 180",
      "released_at": "2024-06-01T00:00:00Z",
      "is_read": false,
      "last_page_read": 0,
      "total_pages": 0
    }
  ],
  "continue_point": {
    "chapter_id": "chapter-42",
    "page": 15
  }
}
```

### `GET /api/history`

Reading history ordered by most recently read.

```json
[
  {
    "manga_id": "manga-123",
    "source_manga_id": "solo-leveling",
    "plugin_id": "mangzio",
    "title": "Solo Leveling",
    "cover_url": "https://example.com/covers/solo-leveling.jpg",
    "total_chapters": 180,
    "read_chapters": 42,
    "last_read_at": "2024-06-15T12:34:56Z"
  }
]
```

### `GET /api/reader-data/{pluginID}/{mangaID}/{chapterID}`

Page list for the reader. **Existing endpoint** — returns `camelCase` keys.

```json
{
  "pages": [
    {"index": 0, "url": "https://example.com/pages/p1.jpg"},
    {"index": 1, "url": "https://example.com/pages/p2.jpg"}
  ],
  "pluginID": "mangzio",
  "mangaID": "solo-leveling",
  "chapterID": "chapter-42",
  "prevChapterID": "chapter-41",
  "nextChapterID": "chapter-43"
}
```

### `POST /api/library/toggle/{pluginID}/{mangaID}`

Add or remove a manga from the library. Returns the new state.

```json
{"in_library": true}
```

### `POST /api/chapters/read/{pluginID}/{mangaID}/{chapterID}`

Mark a chapter as read (or unread). Body is optional — omit to toggle, or send
explicitly:

```json
{"read": true}
```

Response:

```json
{"is_read": true}
```

### `POST /api/progress/{pluginID}/{mangaID}/{chapterID}`

Set the reader's current page position.

**Request:**

```json
{"page": 15}
```

**Response:**

```json
{"page": 15, "total_pages": 42}
```

## Curl Examples

**Without API key** (key not configured):

```bash
curl -s http://127.0.0.1:8080/api/library | jq

curl -s 'http://127.0.0.1:8080/api/search?pluginID=mangzio&q=solo+leveling' | jq

curl -s http://127.0.0.1:8080/api/manga/mangzio/solo-leveling | jq
```

**With API key** (key configured via `-apiKey`):

```bash
curl -s -H 'X-API-Key: my-secret' http://127.0.0.1:8080/api/library | jq

curl -s -H 'X-API-Key: my-secret' \
  'http://127.0.0.1:8080/api/search?pluginID=mangzio&q=isekai&page=1' | jq

curl -s -X POST -H 'X-API-Key: my-secret' \
  http://127.0.0.1:8080/api/library/toggle/mangzio/solo-leveling | jq
```

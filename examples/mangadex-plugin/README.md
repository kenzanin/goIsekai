# mangadex-plugin

A goIsekai manga source plugin that fetches manga, chapters, and pages from the
[MangaDex](https://mangadex.org) public API.

## Build

```bash
make build          # produces mangadex.wasm
```

## Install

```bash
make install        # copies to ../../app_data/plugins/mangadex.wasm
```

## API endpoints

| Function | MangaDex endpoint |
|---|---|
| Search | `GET /manga?title=X&limit=20&offset=N&includes[]=cover_art&order[followedCount]=desc` |
| GetMangaDetail | `GET /manga/{id}?includes[]=cover_art,author,artist` |
| GetChapterList | `GET /manga/{id}/feed?translatedLanguage[]=en&order[chapter]=asc&limit=500` (paginated) |
| GetPageList | `GET /at-home/server/{chapterId}` → pages at `{baseUrl}/data/{hash}/{file}` |

All network access goes through the host proxy (`host_http_request`); the plugin
never opens sockets directly.

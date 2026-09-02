# Dummy JS Plugin

Reference manga-source plugin for goIsekai written in JavaScript and run on
the goja engine. Serves a small hardcoded catalog so it works offline without
any network calls.

## Plugin structure

```
main.js    — entry point with PLUGIN metadata object and ABI functions
```

## ABI contract

JS plugins expose PascalCase functions (the host dispatch table maps the
PascalCase ABI constants directly):

| Function           | Input (JSON string)        | Output (JSON string)         |
|--------------------|----------------------------|------------------------------|
| `Search`           | `{"query":"...","page":1}` | `[{id, title, cover_url}]`   |
| `GetMangaDetail`   | `"<manga-id>"`            | `{id, title, author, ...}`   |
| `GetChapterList`   | `"<manga-id>"`            | `[{id, number, title, ...}]` |
| `GetPageList`      | `"<chapter-id>"`          | `[{index, url}]`             |

## Metadata object

```js
var PLUGIN = {
    contract_version: 1,   // must match host version
    name: "Dummy JS",
    verify_url: "https://example.com",
    needs_human_verify: false,
    thumb_ratio: 0.703,
    search_page_size: 24
};
```

## Available globals

- `JSON.parse` / `JSON.stringify` — JSON helpers
- `http_request(jsonString)` — HTTP via host proxy (returns JSON string)
- `log.debug(msg)` / `log.info(msg)` / `log.warn(msg)` / `log.error(msg)`

## Install

Drop the plugin folder into `app_data/plugins/` and restart the server. The
folder name becomes the plugin ID.

# Dummy Lua Plugin

Reference manga-source plugin for goIsekai written in Lua. Serves a small
hardcoded catalog so it works offline without any network calls.

## Plugin structure

```
main.lua    — entry point with PLUGIN metadata table and ABI functions
```

## ABI contract

Lua plugins expose snake_case globals:

| Function            | Input (JSON string)        | Output (JSON string)         |
|---------------------|----------------------------|------------------------------|
| `search_manga`      | `{"query":"...","page":1}` | `[{id, title, cover_url}]`   |
| `get_manga_detail`  | `"<manga-id>"`            | `{id, title, author, ...}`   |
| `get_chapter_list`  | `"<manga-id>"`            | `[{id, number, title, ...}]` |
| `get_page_list`     | `"<chapter-id>"`          | `[{index, url}]`             |

## Metadata table

```lua
PLUGIN = {
    contract_version = 1,       -- must match host version
    name = "Dummy",
    verify_url = "https://example.com",
    needs_human_verify = false,
    thumb_ratio = 0.70,
    search_page_size = 24
}
```

## Available globals

- `json.encode(value)` / `json.decode(string)` — JSON helpers
- `http_request(req_table)` — HTTP via host proxy
- `log.debug(msg)` / `log.info(msg)` / `log.warn(msg)` / `log.error(msg)`
- `require("sibling")` — load a sibling `.lua` module from the plugin folder

## Install

Drop the plugin folder into `app_data/plugins/` and restart the server.

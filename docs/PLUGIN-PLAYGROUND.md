# Plugin Playground

The sandbox API lets you develop and debug goIsekai plugins entirely from the
terminal — no browser, no server restarts, no copy-paste workflows.

## Quick Start

```bash
# Start the server (plugins load automatically from app_data/plugins/)
./goisekai

# List loaded plugins
curl -s localhost:3333/api/sandbox/plugins/ | jq

# Search
curl -s 'localhost:3333/api/sandbox/plugins/kaliscan/search?q=naruto' | jq

# Detail
curl -s 'localhost:3333/api/sandbox/plugins/kaliscan/detail/solo-leveling' | jq

# Chapters
curl -s 'localhost:3333/api/sandbox/plugins/kaliscan/chapters/solo-leveling' | jq

# Pages (chapter ID uses : separator, not /)
curl -s 'localhost:3333/api/sandbox/plugins/kaliscan/pages/solo-leveling:chapter-1' | jq
```

## Hot Reload Cycle

Edit → reload → test. No server restart needed.

```bash
# 1. Edit your plugin
vim examples/lua/kaliscan/main.lua

# 2. Reload it live
curl -s -X POST localhost:3333/api/sandbox/plugins/kaliscan/reload | jq

# 3. Test immediately
curl -s 'localhost:3333/api/sandbox/plugins/kaliscan/search?q=naruto' | jq
```

## Plugin Lifecycle

```bash
# Load a plugin from an external path (without installing to app_data/)
curl -s -X POST localhost:3333/api/sandbox/plugins/load \
  -H 'Content-Type: application/json' \
  -d '{"path":"/home/you/my-plugin/main.lua"}' | jq
# → {"id":"my-plugin"}

# Unload
curl -s -X POST localhost:3333/api/sandbox/plugins/my-plugin/unload | jq
# → {"status":"unloaded"}

# Reload (unload + load from same path)
curl -s -X POST localhost:3333/api/sandbox/plugins/my-plugin/reload | jq
# → {"id":"my-plugin"}
```

## Full API Reference

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/sandbox/plugins/` | List all loaded plugins with metadata |
| `POST` | `/api/sandbox/plugins/load` | Load plugin from external path `{"path":"..."}` |
| `POST` | `/api/sandbox/plugins/{id}/unload` | Unload a plugin |
| `POST` | `/api/sandbox/plugins/{id}/reload` | Reload from same path |
| `GET` | `/api/sandbox/plugins/{id}/search?q=...&page=1` | Search manga |
| `GET` | `/api/sandbox/plugins/{id}/detail/{mangaID}` | Get manga details + chapter list |
| `GET` | `/api/sandbox/plugins/{id}/chapters/{mangaID}` | Get chapter list |
| `GET` | `/api/sandbox/plugins/{id}/pages/{chapterID}` | Get page image URLs |

## Writing a New Plugin

Pick your runtime:

| Runtime | Language | File | Build step |
|---------|----------|------|------------|
| **Lua** | Lua 5.1 | `main.lua` | None — drop in folder |
| **JS** | ES5.1 | `main.js` | None — drop in file |
| **WASM** | Go → wasm | `main.go` | `GOOS=wasip1 GOARCH=wasm go build` |

### Plugin Metadata

Every plugin must declare a global `PLUGIN` table (Lua/JS) or export
`contract_version` (WASM). Required fields:

```
contract_version: 1       -- must be 1
name: "My Source"         -- display name
verify_url: "https://..." -- site URL for human verification
needs_human_verify: false
thumb_ratio: 0.70         -- cover aspect ratio (width/height)
search_page_size: 24      -- results per page (default 24)
```

Add `needs_js: true` if the source requires a browser for anti-bot challenges
(the host will attempt CDP fallback automatically).

### ABI Contract

All runtimes implement 4 functions with identical JSON shapes:

**`Search(arg)`** — `arg`: `{"query":"...","page":1}`
Returns: `[{"id":"...", "title":"...", "cover_url":"..."}]`

**`GetMangaDetail(arg)`** — `arg`: `"manga-id"` (JSON string)
Returns: `{"id":"...", "title":"...", "author":"...", "description":"...",
           "cover_url":"...", "genres":["..."], "status":"ongoing"}`

**`GetChapterList(arg)`** — `arg`: `"manga-id"` (JSON string)
Returns: `[{"id":"...", "manga_id":"...", "title":"...", "chapter_num":1,
           "released_at":"...", "url":"..."}]`

**`PageList(arg)`** — `arg`: `"manga-id:chapter-N"` (JSON string)
Returns: `[{"index":0, "url":"https://...", "headers":{}}]`

The `headers` object lets a plugin forward required headers (e.g. `Referer`)
for image CDNs that check origin.

### Available Host Functions

**`http_request(jsonString)`** — HTTP client with browser TLS fingerprint
(`Chrome_146` profile), automatic cookie jar, and pacing.

```json
{
  "method": "GET",
  "url": "https://example.com/api/search?q=naruto",
  "headers": {"Referer": "https://example.com/"},
  "body": "",
  "timeout": 30
}
```

Returns `{"status": 200, "headers": {...}, "body": "..."}`

**`log.debug/info/warn/error(msg)`** — Logs visible at `/view/logs` and in
sandbox responses.

### Lua-specific

- `json.encode(obj)` / `json.decode(str)` — JSON conversion
- `require("util")` — loads sibling `.lua` files from the same folder
- Sandbox: only `base`, `string`, `table`, `math` stdlib (no `os`, `io`, `debug`)

### JS-specific

- `JSON.parse()` / `JSON.stringify()` — native
- `http_request(jsonString)` — same as Lua
- `log.debug/info/warn/error(msg)` — same as Lua
- ES5.1 only (no `let`, `const`, arrow functions, template literals, `Promise`)

### WASM-specific

- Uses Extism PDK (`github.com/extism/go-pdk`)
- Import host function: `//go:wasmimport extism:host/user host_http_request`
- Export: `//go:wasmexport Search` (PascalCase)
- Build: `GOOS=wasip1 GOARCH=wasm go build -o plugin.wasm .`
- Install: copy `.wasm` to `app_data/plugins/` or use `POST /load`

## Development Workflow

### 1. Start with the dummy plugin

```bash
cp -r examples/lua/dummy app_data/plugins/dummy-lua
# or
cp examples/js/dummy/main.js app_data/plugins/dummy-js.js
```

### 2. Test baseline

```bash
curl -s 'localhost:3333/api/sandbox/plugins/dummy-lua/search?q=test' | jq
```

### 3. Implement real logic

Replace hardcoded catalog with `http_request` calls to the target site.

### 4. Iterative debug cycle

```bash
# Edit → reload → test (repeat every 5 seconds)
vim app_data/plugins/dummy-lua/main.lua
curl -s -X POST localhost:3333/api/sandbox/plugins/dummy-lua/reload | jq
curl -s 'localhost:3333/api/sandbox/plugins/dummy-lua/search?q=naruto' | jq
```

### 5. Check logs

Plugin `log.info/warn/error` calls appear in the response at `/view/logs`.
Filter by plugin: `curl 'localhost:3333/view/logs?filter=plugins'`

### 6. Install when ready

```bash
# Copy to plugin folder
cp my-plugin/main.lua app_data/plugins/my-plugin/main.lua

# Or use the hot-load API
curl -s -X POST localhost:3333/api/sandbox/plugins/load \
  -d '{"path":"my-plugin/main.lua"}'
```

## Tips

- **Cover images**: some CDNs require `Referer` header. Set it in the `headers`
  field of each page URL — the host forwards it automatically.
- **Rate limiting**: the host paces HTTP requests per-host (~1 req/s). Don't
  add your own delays.
- **Search pagination**: declare `search_page_size` in your plugin metadata if
  it differs from the default 24. The host uses this to render Next/Prev links.
- **Anti-bot sites**: set `needs_js: true` in metadata. The host will attempt
  CDP fallback (lightpanda/chrome/obscura) when tls-client gets blocked.
- **Chapter ID format**: use `:` as separator (`manga-slug:chapter-42`), not
  `/`. The host routes on `:` in chapter IDs.

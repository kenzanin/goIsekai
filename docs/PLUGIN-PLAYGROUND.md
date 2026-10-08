# Plugin Playground

The sandbox API lets you develop and debug goIsekai plugins entirely from the
terminal — no browser, no server restarts, no copy-paste workflows.

## Quick Start

```bash
# Start the server (plugins load automatically from app_data/plugins/)
./goisekai

# List loaded plugins
curl -s localhost:8080/api/sandbox/plugins/ | jq

# Search
curl -s 'localhost:8080/api/sandbox/plugins/kaliscan/search?q=naruto' | jq

# Detail
curl -s 'localhost:8080/api/sandbox/plugins/kaliscan/detail/solo-leveling' | jq

# Chapters
curl -s 'localhost:8080/api/sandbox/plugins/kaliscan/chapters/solo-leveling' | jq

# Pages (chapter ID uses : separator, not /)
curl -s 'localhost:8080/api/sandbox/plugins/kaliscan/pages/solo-leveling:chapter-1' | jq
```

## Hot Reload Cycle

Edit → reload → test. No server restart needed.

```bash
# 1. Edit your plugin
vim examples/plugins/lua/kaliscan/main.lua

# 2. Reload it live
curl -s -X POST localhost:8080/api/sandbox/plugins/kaliscan/reload | jq

# 3. Test immediately
curl -s 'localhost:8080/api/sandbox/plugins/kaliscan/search?q=naruto' | jq
```

## Plugin Lifecycle

```bash
# Load a plugin from an external path (without installing to app_data/)
curl -s -X POST localhost:8080/api/sandbox/plugins/load \
  -H 'Content-Type: application/json' \
  -d '{"path":"/home/you/my-plugin/main.lua"}' | jq
# → {"id":"my-plugin"}

# Unload
curl -s -X POST localhost:8080/api/sandbox/plugins/my-plugin/unload | jq
# → {"status":"unloaded"}

# Reload (unload + load from same path)
curl -s -X POST localhost:8080/api/sandbox/plugins/my-plugin/reload | jq
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

> Note: Sandbox routes are under `/api/` and require an API key if configured
> (via `-apiKey` flag or `api_key` in `goisekai.ini`). When no key is set,
t> the routes are unauthenticated.

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

### Shared helpers (`host.*`) — Lua and JS

- `host.json.decode(str)` — JSON string to a value; `host.json.encode(value)` —
  value back to a JSON string. Both runtimes use the same codec, so results and
  error text match.
- `host.text.*` — url/html decode, strip html/markdown, titlecase, trim, and more
- `host.codecs.*` — base64, base64url and hex encode/decode
- `host.crypto.*` — sha256, md5, hmac-sha256, xor, utf8 hex, vrf_sign, and
  `aes_gcm_decrypt(key, iv, tag, ciphertext)` (AES-256-GCM; all four arguments
  base64url, returns the plaintext, throws/returns nil plus a message on a bad
  key or tag). The tag is a separate argument, so callers porting from
  WebCrypto or node split it off the ciphertext themselves.
- `host.crypto.substitute_cipher(data, material, direction)` — byte-wise
  substitution cipher, `direction` being `"encrypt"` or `"decrypt"` (exact
  inverses, so `decrypt(encrypt(x)) == x`). Each byte goes through
  `sbox[data[i] ^ key[i % len(key)] ^ prev]`, with the substituted byte carried
  forward as `prev` for the next position, applied over `rounds` rounds.
  `material` is JSON: `{"sboxes": [[256 ints], ...], "keys": [[ints], ...],
  "previous": [ints]}`, one entry per round. Ships in the host because neither
  runtime can express it — Lunar Lua has no bitwise operators at all, and sites
  that sign their API this way put the tables in their own front-end bundle, so
  the plugin is the only layer that has read them. Store the material in a
  sibling data file; a sbox that is not a permutation is rejected with a message
  rather than producing a token the site answers with a bare `invalid_token`.
- `host.http.get(url, headers?)` / `host.http.post(url, headers?, body)` — same
  transport as `http_request`
- `host.browser.fetch(url)` — navigates to `url` in a real browser (CDP engine),
  waits for client-side JavaScript to run and any anti-bot challenge to clear,
  and returns the rendered HTML, or nil/null when no engine is configured.
  For sites whose API is signed by their own front-end code, this is the only
  way to reach data that never appears in static HTML.
- `host.browser.evaluate(url, js)` — navigates to `url`, runs `js` (a function
  expression like `() => document.title`) in the page context, and returns the
  result as a string. Use it to call the site's own functions or to extract
  data from the live DOM.

### Lua-specific

- `require("util")` — loads sibling `.lua` files from the same folder
- Sandbox: only `base`, `string`, `table`, `math` stdlib (no `os`, `io`, `debug`)
- There is **no** global `json` table; use `host.json.*`

### JS-specific

- `JSON.parse()` / `JSON.stringify()` — the VM's own codec, kept for local work;
  prefer `host.json.*` when a plugin must behave identically to a Lua plugin
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
cp -r examples/plugins/lua/dummy app_data/plugins/dummy-lua
# or
cp examples/plugins/js/dummy/main.js app_data/plugins/dummy-js.js
```

### 2. Test baseline

```bash
curl -s 'localhost:8080/api/sandbox/plugins/dummy-lua/search?q=test' | jq
```

### 3. Implement real logic

Replace hardcoded catalog with `http_request` calls to the target site.

### 4. Iterative debug cycle

```bash
# Edit → reload → test (repeat every 5 seconds)
vim app_data/plugins/dummy-lua/main.lua
curl -s -X POST localhost:8080/api/sandbox/plugins/dummy-lua/reload | jq
curl -s 'localhost:8080/api/sandbox/plugins/dummy-lua/search?q=naruto' | jq
```

### 5. Check logs

Plugin `log.info/warn/error` calls appear in the response at `/view/logs`.
Filter by plugin: `curl 'localhost:8080/view/logs?filter=plugins'`

### 6. Install when ready

```bash
# Copy to plugin folder
cp my-plugin/main.lua app_data/plugins/my-plugin/main.lua

# Or use the hot-load API
curl -s -X POST localhost:8080/api/sandbox/plugins/load \
  -d '{"path":"my-plugin/main.lua"}'
```

## Tips

- **Cover images**: some CDNs require `Referer` header. Set it in the `headers`
  field of each page URL — the host forwards it automatically.
- **Rate limiting**: the host paces HTTP requests per-host (~1 req/s). Don't
  add your own delays.
- **Search pagination**: return every match and let the host slice the
  results into pages of 30. Don't paginate inside the plugin.
- **Anti-bot sites**: set `needs_js: true` in metadata. The host will attempt
  CDP fallback (lightpanda/chrome/obscura) when tls-client gets blocked.
- **Chapter ID format**: use `:` as separator (`manga-slug:chapter-42`), not
  `/`. The host routes on `:` in chapter IDs.

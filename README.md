# goIsekai

A self-hosted manga reader with sandboxed plugins. Four runtimes — hardened **WASM**, zero-toolchain **Lua**, pure-Go **JS** (goja), and native **Go** (.so) — power your sources; one fast server-rendered UI reads them all.

goIsekai is a single static Go binary that serves a chi + Jet + HTMX + Tailwind web UI. Manga sources are plugins executed in isolated sandboxes, so a crashing or malicious plugin can never take down the host. All network traffic goes through a Chrome-fingerprinted TLS client, so sites behind Cloudflare just work — with an automatic browser-fallback (CDP) when a challenge appears anyway.

![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)

## Architecture

```mermaid
flowchart LR
    subgraph Browser
        UI[HTMX + Tailwind UI]
        Reader[SPA Canvas Reader]
    end

    subgraph goIsekai[goIsekai single binary]
        HTTP[chi HTTP server]
        API[/api JSON endpoints]
        Bridge[AppService bridge]
        PM[pluginmanager<br/>lazy-load]
        subgraph Sandboxes
            WASM[Extism WASM<br/>64 MB / 15 s]
            Lua[lunar<br/>safe stdlib]
            JS[goja<br/>ES5.1]
            GO[go plugin<br/>.so]
        end
        HostNet[hostnet proxy<br/>tls-client + CDP fallback]
        DB[(SQLite<br/>modernc.org)]
        Cache[(WebP disk cache)]
    end

    subgraph Internet
        Sites[Manga sites]
        CDP[CDP browser fallback<br/>lightpanda / chrome]
    end

    UI --> HTTP
    Reader --> HTTP
    HTTP --> Bridge
    HTTP --> API
    API --> Bridge
    Bridge --> PM
    PM --> WASM
    PM --> Lua
    PM --> JS
    PM --> GO
    WASM --> HostNet
    Lua --> HostNet
    JS --> HostNet
    HostNet -->|Chrome_146 TLS| Sites
    HostNet -.->|403/503 challenge| CDP
    CDP -.->|solved cookies| HostNet
    Bridge --> DB
    Bridge --> Cache
```

## Features

- **Four plugin runtimes** — sandboxed **WASM** (64 MB cap, 15 s timeout, panic isolation), **Lua** (lunar, plain text, no toolchain), **JS** (goja, ES5.1, JSON native), and native **Go** (.so). All share one ABI; plugins are lazy-loaded on first use
- **API-first** — every feature has a JSON endpoint under `/api` with constant-time API-key auth; the HTML UI and any future client consume the same bridge
- **Browser TLS fingerprinting** — `bogdanfinn/tls-client` with a Chrome profile + per-plugin cookie jars; built for adversarial sites
- **Automatic anti-bot fallback** — when a site returns a Cloudflare challenge, the host spawns a CDP browser (lightpanda or Chrome), solves it, harvests the cookies back into the jar, and retries the fast path — no manual paste
- **SPA reader** — canvas reader with fetch-swap chapter navigation (no page reload), cursor-anchored zoom, drag pan, fit-width/fit-height/1:1 modes, RTL/LTR, keyboard nav (arrows/space/Esc/Home/End/r/PageUp/Down), per-chapter read progress, read-ahead prefetch into the next chapter
- **Read tracking** — per-chapter progress (`N/M` pages), strikethrough when a chapter is fully read or manually marked, reset buttons, cached-page counts, continue-from-history
- **Alt-titles enricher** — plugins can declare alt-title servers (MangaDex, MAL, AI, …); host discovers them by capability, fetches alternative titles, and lets you swap the main title with any result
- **Library stats** — title counts by status (done/ongoing/unknown), finished/reading/total, estimated time spent, most/fewest chapters
- **FTS5 library search** — full-text search across titles and alt-titles with Go-side fuzzy ranking (exact > prefix > substring > subsequence)
- **CBZ export** — per-manga or per-chapter export from the disk cache as an ordered ZIP (1.webp, 2.webp, …); works offline for fully-read chapters
- **Live logs** — merged app + plugin logs over WebSocket, filterable, selectable, copyable, clearable
- **Brotli precompression** — static JS/CSS served as `.br` when the client accepts it
- **Single static binary** — pure Go, `CGO_ENABLED=0`, cross-compiles to Linux/Windows/macOS trivially

## Quick start

```sh
make build          # CGO-free build -> ./goisekai
./goisekai          # serves http://localhost:8080 (add -open to launch a browser)
```

Host/port come from CLI flags or `goisekai.ini` (flags win):

```sh
./goisekai -host 127.0.0.1 -port 8080 -logLevel debug -apiKey your-secret
```

## Plugins

Plugins implement a small ABI (`Init`, `SearchManga`, `GetMangaDetails`, `GetChapterList`, `GetPageList`, optional `GetAltTitles`) and call the host function `http_request` for all networking. All runtimes are interchangeable — pick WASM for hardened plugins, Lua for quick ones, JS for JSON-heavy ones.

### WASM plugins

```sh
tinygo build -o plugins/mangadex.wasm -target wasm ./plugins/mangadex/
```

Install a `.wasm` from the Plugins screen in the UI. See `examples/plugins/wasm/mangadex/` for a complete TinyGo plugin.

### Lua plugins (no toolchain needed)

One folder per site under `plugins/lua/<id>/`, with `main.lua` as the entry point; sibling modules are pre-loaded and loadable via `require("module")` (sandboxed to the plugin folder).

```lua
local PLUGIN = {
  name = "KaliScan",
  version = "1.0.0",
  thumb_ratio = 0.71,
}

function search_manga(filter_json)
  local filter = json.decode(filter_json)
  local res = http_request({ url = "https://kaliscan.io/?s=" .. filter.query })
  return results
end
```

`main.lua` declares a `PLUGIN` table and four globals (`search_manga`, `get_manga_detail`, `get_chapter_list`, `get_page_list`). Each takes one JSON-string argument and returns a Lua table. Networking goes through `http_request({url=..., method=..., headers=...})`, which rides the same TLS-fingerprinted, cookie-jarred, rate-paced session as WASM plugins. `json.encode`/`json.decode` are provided. Available stdlib: `string`, `table`, `math`, `os.time/date/clock` — no `io`, no `os.execute`. See `examples/plugins/lua/kaliscan/` for a complete example.

### JS plugins (no toolchain needed)

One folder per site under `plugins/js/<id>/`, with `main.js` as the entry point.

```js
var PLUGIN = {
  name: "MangaDex",
  version: "1.0.0",
  thumb_ratio: 0.71,
  alt_title_servers: [{ id: "mangadex", name: "MangaDex" }],
};

function search_manga(filterJson) {
  var filter = JSON.parse(filterJson);
  var res = JSON.parse(http_request(JSON.stringify({ url: "https://api.mangadex.org/manga?title=" + encodeURIComponent(filter.query) })));
  return res.data;
}
```

Same ABI as Lua — `PLUGIN` metadata + four PascalCase globals. `http_request` takes a JSON string, returns a JSON string. `json` is native JS. See `examples/plugins/js/mangadex/` for a complete example.

```sh
make install-plugins   # copies all plugin sources → app_data/plugins/
```

## Anti-bot fallback (CDP)

Sites protected by Cloudflare Turnstile / DataDome can challenge the fast client. When `hostnet` detects a challenge (403/503 + marker), it falls back to a CDP-driven browser:

- `cdp_engine` — `lightpanda` (light, ~9× less memory) or `chrome` (most complete); `off` disables the fallback
- `cdp_path` — path to the browser binary

The flow: challenge detected → spawn browser → navigate & solve → harvest cookies into the plugin's jar → retry the original request. Solved cookies are scoped to the target host and return to the fast path for all subsequent requests. When the browser can't solve it, the UI shows the human-verify banner instead.

Plugins can hint `needs_js = true` in their metadata to skip the fast path and go straight to the browser.

## API

All features are available as JSON endpoints under `/api`. Pass `-apiKey` to require an API key (constant-time comparison, 401 for all probes).

```sh
curl -H "X-API-Key: your-secret" http://localhost:8080/api/search?pluginID=mangadex&q=isekai
```

See `docs/API.md` for the full endpoint reference.

## Development

```sh
make build           # build the server
make test            # go test ./...
make lint            # golangci-lint
make lint-web        # Biome (web sources)
make check           # fmt + test + modernize + lint
make install-plugins # install example plugins
make br              # brotli-compress static assets
make clean           # remove binary + generated assets
```

Stack: Go 1.27 · chi · CloudyKit/jet · HTMX · Tailwind (static build) · Extism · lunar · goja · tls-client · chromedp · modernc.org/sqlite (via go-jet) · gen2brain/webp.

## License

[MIT](LICENSE)

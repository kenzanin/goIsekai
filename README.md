# goIsekai

A self-hosted manga reader with sandboxed WebAssembly plugins.

goIsekai runs a local HTTP server and serves a fast server-rendered web UI (chi + Jet templates + HTMX + Tailwind). Manga sources are **WASM plugins** executed in a hardened wazero sandbox — a crashing or malicious plugin can never take down the host. All network traffic goes through a Chrome-fingerprinted TLS client, so sites behind Cloudflare just work.

![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)

## Features

- **WASM plugin sources** — sandboxed (64 MB cap, 15 s timeout, panic isolation), language-agnostic (TinyGo today, any WASM target works)
- **Browser TLS fingerprinting** — bogdanfinn/tls-client with a Chrome profile + per-plugin cookie jars; built for adversarial sites
- **Human verification fallback** — for Cloudflare-challenged sites: paste the `cf_clearance` cookie + browser UA once, the host seeds it into the plugin's cookie jar
- **Reading experience** — HTML5 canvas reader with cursor-anchored zoom, drag pan, fit-width/fit-height/1:1 modes, RTL/LTR, click zones, keyboard nav, per-chapter progress, Suwayomi-style read-ahead that spills into the next chapter
- **Disk cache in WebP** — fetched images are converted to WebP (quality 85) on write, ~45% smaller; laid out per `plugin/manga/chapter`
- **Per-plugin thumbnail ratios** — plugins declare their site's cover aspect ratio so nothing is cropped
- **Live logs** — merged app + request logs at `/#/logs` (selectable, copyable, clearable)
- **Single static binary** — pure Go, `CGO_ENABLED=0`, cross-compiles to Linux/Windows/macOS trivially

## Quick start

```sh
make build          # CGO-free build -> ./goisekai
./goisekai          # serves http://localhost:8080 (add -open to launch a browser)
```

Host/port come from CLI flags or `goisekai.ini` (flags win):

```sh
./goisekai -host 127.0.0.1 -port 8080 -logLevel debug
```

## Plugins

Plugins are WASM modules implementing a small ABI (`Init`, `SearchManga`, `GetMangaDetails`, `GetChapterList`, `GetPageList`) and calling host functions such as `host_http_request`. See `examples/mangadex/` for a complete TinyGo plugin.

```sh
tinygo build -o plugins/mangadex.wasm -target wasm ./plugins/mangadex/
```

Install a `.wasm` from the Plugins screen in the UI.

## Development

```sh
make build      # build the server
make test       # go test ./...
make lint-web   # Biome (web sources)
make clean      # remove binary + generated assets
```

Stack: Go 1.27 · chi · CloudyKit/jet · HTMX · Tailwind (static build) · wazero · tls-client · modernc.org/sqlite (via go-jet) · gen2brain/webp.

## License

[MIT](LICENSE)

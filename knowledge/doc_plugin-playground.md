---
type: reference
title: Plugin Playground
description: Sandbox API for developing and debugging goIsekai plugins from the terminal.
tags: [goisekai, plugins, sandbox, reference]
generated: { by: agent/mcp, at: "2026-10-26T20:15:01Z" }
status: stable
---

# Plugin Playground

Sandbox API for developing and debugging goIsekai plugins from the terminal — no browser, no server restarts, no copy-paste.

## Quick Start

```bash
./goisekai
curl -s localhost:8080/api/sandbox/plugins/ | jq
curl -s 'localhost:8080/api/sandbox/plugins/1manga/search?q=isekai' | jq
curl -s 'localhost:8080/api/sandbox/plugins/1manga/detail/mondaijitachi-ga-isekai-kara-kuru-sou-desu-yo' | jq
curl -s 'localhost:8080/api/sandbox/plugins/1manga/chapters/mondaijitachi-ga-isekai-kara-kuru-sou-desu-yo' | jq
curl -s 'localhost:8080/api/sandbox/plugins/1manga/pages/mondaijitachi-ga-isekai-kara-kuru-sou-desu-yo:chapter-18' | jq
```

Chapter IDs use `:` separator (manga-id:chapter-N), not `/`.

## Hot Reload Cycle

```bash
# 1. Edit → 2. Reload → 3. Test
curl -s -X POST localhost:8080/api/sandbox/plugins/1manga/reload | jq
curl -s 'localhost:8080/api/sandbox/plugins/1manga/search?q=isekai' | jq
```

## Plugin Lifecycle

```bash
curl -s -X POST localhost:8080/api/sandbox/plugins/load -H 'Content-Type: application/json' -d '{"path":"/home/you/my-plugin/main.wasm"}' | jq
# → {"id":"my-plugin"}
curl -s -X POST localhost:8080/api/sandbox/plugins/my-plugin/unload | jq
# → {"status":"unloaded"}
```

## Available Host Functions

- `http_request(jsonString)` — browser TLS fingerprint, cookie jar, pacing
- `log.debug/info/warn/error(msg)` — visible in /view/logs + sandbox responses
- `host.json.*`, `host.text.*`, `host.codecs.*`, `host.crypto.*`, `host.browser.*`

## ABI Contract (4 functions, JSON in/out)

- `Search(arg)` — arg `{"query":"...","page":1}` → `[{"id","title","cover_url"}]`
- `GetMangaDetail(arg)` — arg `"manga-id"` → `{"id","title","author","description","cover_url","genres","status"}`
- `GetChapterList(arg)` — arg `"manga-id"` → `[{"id","manga_id","title","chapter_num","released_at","url"}]`
- `GetPageList(arg)` — arg `"manga-id:chapter-N"` → `[{"index","url","headers"}]`

## Building WASM plugins

WASM plugins build via the plugin's Makefile (TinyGo 0.43.0): `cd examples/plugins/wasm/<id> && make build`. Never `tools/build-wasm.sh` (falls back to std-Go wasip1 → 3.4MB → OOMs wasm runtime). Install by copying the `.wasm` to `app_data/plugins/<id>/main.wasm`.

## Development Workflow

### Writing a new WASM plugin

```bash
# Copy the 1manga template
mkdir -p app_data/plugins/myplugin
cp examples/plugins/wasm/1manga/*.go examples/plugins/wasm/1manga/Makefile examples/plugins/wasm/1manga/plugin.json app_data/plugins/myplugin/
cp examples/plugins/wasm/1manga/main.wasm app_data/plugins/myplugin/main.wasm
curl -s -X POST localhost:8080/api/sandbox/plugins/myplugin/reload | jq
```

## Tips

- Clear `plugin_cache` after plugin edits: `DELETE FROM plugin_cache WHERE plugin_id='<id>';` then reload.
- Plugin invocations run on `context.Background()+15s` — SIGTERM never reaches plugin code.
- Set `needs_js: true` + `manual_cookies: true` for anti-bot sites (host handles CDP or cookie wizard).

# goIsekai — Agent Guide

## Tool Preferences (MANDATORY)

**Always use these tools in this order of priority:**

| Task                                         | Tool                 | Notes                                                            |
| -------------------------------------------- | -------------------- | ---------------------------------------------------------------- |
| Shell commands, builds, tests                | `bash`               | Default for all terminal operations                              |
| Code exploration, find references, relations | `codebase-memory`    | Use `search_graph`, `trace_path`, `get_code_snippet` before grep |
| Memory management, session state             | `agentic-memory-mcp` | Store/recall context across sessions                             |
| Browse web pages, check UI                   | `obscura`            | Primary browser tool                                             |
| Debug web pages (console, network)           | `playwright-cdp`     | Only when deep debugging needed                                  |
| Look up API docs, library info                 | `deepwiki`           | For GitHub repos and documentation                               |

**Workflow:**

1. Before editing code → `codebase-memory` to find all references and callers
2. Before testing changes → `obscura` to verify UI behavior
3. After completing task → `vestige-mcp` to store session context
4. When stuck on API → `deepwiki` to check documentation

---

## Project Overview

**goIsekai** is a manga library manager and reader with an embedded HTTP server and browser UI. Plugins (Lua, JS, Go, or Yaegi) fetch manga from external sites. The host manages libraries, reading progress, image caching, and alt-title enrichment.

Module: `goisekai` · Go 1.27 · CGO-free · pure Go SQLite

---

## Commands

Recipes live in the `justfile` (`just --list`).

| Command          | What it does                                                               |
| ---------------- | -------------------------------------------------------------------------- |
| `just build`     | Full build (runs `css`, `br`, then `CGO_ENABLED=0 go build`)               |
| `just test`      | Run all tests (`CGO_ENABLED=0 go test ./internal/... ./pkg/... ./cmd/...`) |
| `just race`      | Run tests with `-race` (`CGO_ENABLED=1`)                                   |
| `just lint`      | `golangci-lint run ./internal/... ./pkg/... ./cmd/...`                     |
| `just modernize` | `modernize -fix` on all packages                                           |
| `just check`     | Production-Go gate: fmt + modernize + lint, skipping tests and Lua/web     |
| `just run`       | Build + launch (`./goisekai -logLevel debug`); args pass through           |
| `just devrun`    | Alias of `just run`                                                        |
| `just fmt`       | `go fmt` on all packages                                                   |
| `just fmt-prod`  | `go fmt` on production packages (the `check` scope)                        |
| `just lint-prod` | `golangci-lint run --tests=false` on production packages                   |
| `just fmt-web`   | `biome check --write cmd/goisekai/frontend`                                |
| `just fmt-lua`   | `stylua internal/templates/`                                               |
| `just lint-web`  | `biome check cmd/goisekai/frontend` (read-only)                            |
| `just lint-lua`  | `luacheck internal/templates/ --codes --no-unused --no-unused-args`        |
| `just install-lua kaliscan` | Copy a Lua plugin into the plugins dir                           |
| `just install-info mangadex` | Copy an info script into the info dir                           |

All Go commands use `CGO_ENABLED=0` by default (pure Go SQLite). Tests in `just race` set `CGO_ENABLED=1`.

---

## Architecture

```
cmd/goisekai/main.go          ← wiring: config → db → proxy → plugin manager → server
internal/bridge/service.go    ← ApplicationService — glues DB, plugin manager, proxy into a single API
internal/httpserver/*         ← HTTP routes, views (Lua templates), API handlers, actions
internal/database/*           ← SQLite (modernc.org/sqlite), go-jet DSL queries (.gen/)
internal/pluginmanager/*      ← Plugin loading (Lua/lunar, JS/goja, Go plugin, Yaegi)
internal/hostnet/*            ← HTTP proxy, CDP browser engine for anti-bot solving
internal/enrich/*             ← Enrichment registry (providers come from info scripts)
internal/templates/*          ← Lua templates for HTML rendering
internal/config/*             ← Hand-rolled INI parser (goisekai.ini)
internal/bridge/cache, image  ← Image caching and download pipeline
pkg/types/*                   ← Plugin ABI contract, shared types
```

**Data flow for a plugin search:**

1. HTTP handler → `AppService.Search()` → `PluginManager.Search()` → instantiate plugin VM → call `Search()` export → parse JSON → return

**Data flow for reading:**

1. Reader loads chapters from DB → plugin's `GetPageList()` → pages downloaded → cached on disk → served through image proxy

**Plugin ABI** (`pkg/types/abi.go`): Plugins export `Search`, `GetMangaDetail`, `GetChapterList`, `GetPageList` as JSON-over-string functions. Optional: `Init`, `GetEnrichment`. The host imports `host_http_request` for all network access.

**Info scripts** are metadata-only Lua scripts in the info directory (`app_data/info/<source>/main.lua`), discovered separately from source plugins and hidden from the plugin list. They declare enrichment providers in `PLUGIN.enrichment_providers` and implement only `getEnrichment`.

---

## Database

- SQLite via `modernc.org/sqlite` (pure Go, no CGO)
- Queries use **go-jet** DSL in `internal/database/.gen/` — never hand-write SQL in production code (migrations are the exception)
- Tables: `mangas`, `chapters`, `chapters_pages`, `plugins`, `reading_history`, `alt_titles`
- FTS5 virtual table `library_fts` powers library search
- Migrations: `internal/database/schema.go` defines a `migrations` string slice; `runMigrations()` in `db.go` applies pending ones via `PRAGMA user_version`
- DB path: `<data_dir>/goisekai.db` (data_dir from config, default `app_data`)
- WAL mode enabled, busy timeout 5000ms

**Jet DSL pattern** (from `chapters_query.go`):

```go
Chapters.INSERT(Chapters.ID, Chapters.MangaID, ...).Exec(d.db)
SELECT(Chapters.Title).FROM(Chapters).WHERE(Chapters.ID.EQ(String(id))).Query(d.db, &out)
```

Note the dot-import of `. "github.com/go-jet/jet/v2/sqlite"` in query files — it's intentional and excluded from staticcheck.

---

## Plugin System

Four plugin kinds supported by `PluginManager`:

| Kind    | Runtime           | Entry       | Notes                                         |
| ------- | ----------------- | ----------- | --------------------------------------------- |
| `lua`   | lunar VM          | `main.lua`  | Full Lua 5.4, host functions injected         |
| `js`    | goja VM           | `main.js`   | ES5-compatible, host functions injected       |
| `go`    | `plugin.Load`     | `.so`       | Native Go plugin, must match host ABI exactly |
| `yaegi` | Yaegi interpreter | `.go` files | Go-like dialect, interpreted                  |

Plugins are **lazily loaded**: first invocation instantiates the VM, subsequent calls reuse it. Each plugin is protected by a `sync.Mutex` to prevent interleaved invocations.

Invoke timeout: **15 seconds** per plugin call.

Plugin network calls route through `hostnet.Proxy` which handles TLS fingerprinting, CDP challenge solving, and default headers.

---

## Frontend

- **Templates**: Lua + HTML in `internal/templates/` (`layouts/`, `views/`, `partials/`)
- **Template engine**: `internal/templates/lua_engine.go` — renders Lua templates with a custom engine (h function for HTML, host functions for data)
- **Styles**: Tailwind CSS 3.4, compiled to `cmd/goisekai/frontend/lib/tailwind.css` (brotli-compressed)
- **JS**: Alpine.js + custom components in `cmd/goisekai/frontend/lib/alpine-components.js`
- **SPA routing**: `X-Partial: true` header returns only `<main>` content (no layout wrapper)
- **Static assets**: Served from `cmd/goisekai/frontend/`, embedded via Go embed or disk path per config

---

## Config

- File: `goisekai.ini` (auto-generated on first run) or `$GOISEKAI_CONFIG`
- Sections: `[app]`, `[network]`, `[maintenance]`
- CLI flags override config: `-logLevel`, `-host`, `-port`, `-cdpEngine`, `-cdpPath`, `-apiKey`
- **Hot reload**: `goisekai.ini` is polled every 5s; safe fields (log level, user-agent, referer) apply live
- Config paths default relative to working directory

---

## Testing

- Tests live alongside source as `*_test.go`
- Test data fixtures: `internal/pluginmanager/testdata/`
- Run: `just test` (CGO_ENABLED=0) or `just race` (CGO_ENABLED=1 + `-race`)
- E2E CDP test: `internal/hostnet/cdp_e2e_test.go` (requires browser)
- Test files for bridge, database, pluginmanager are all in their respective packages

---

## Gotchas & Conventions

- **Always `CGO_ENABLED=0`** for builds unless testing race conditions. The SQLite driver is pure Go.
- **Plugin files** live in `<data_dir>/plugins/` at runtime (default `app_data/plugins`). Lua plugins are directories with `main.lua`; Go plugins are `.so` files.
- **go-jet dot imports** (`. "github.com/go-jet/jet/v2/sqlite"`) are in query files only (`*_query.go`). The golangci-lint config explicitly excludes this.
- **Template naming**: `views/` files are page templates (full layouts); `partials/` are sub-templates included via Lua `require` or inline rendering.
- **Image cache**: On-disk under `<cache_dir>/images/`. Pages are keyed by a deterministic hash.
- **PID file**: Written to `<data_dir>/goisekai.pid`, removed on shutdown.
- **Database maintenance**: Automatic orphan pruning at startup, periodic DB backups to `<data_dir>/backups/`.
- **Plugin static files**: `plugin_static.go` serves plugin assets (images, etc.) under `/plugin_static/`.
- **Biome** is used for frontend formatting/linting (not ESLint/Prettier). **Stylua** for Lua templates. **Luacheck** for Lua linting.

---

## Change Management (OpenSpec)

Changes use the **openspec** workflow in `openspec/`:

- `openspec/specs/` — current specs
- `openspec/changes/` — active change proposals (delta specs)
- `openspec/changes/archive/` — completed changes
- Commands via `.opencode/commands/opsx-*`: `opsx-explore`, `opsx-propose`, `opsx-apply`, `opsx-archive`, `opsx-sync`, `opsx-update`

---

## MCP Server Tools Reference

### codebase-memory

```bash
# List projects
mcp_codebase-memory_list_projects

# Index repository
mcp_codebase-memory_index_repository \
  --repo_path /path/to/repo \
  --mode full \
  --name project-name

# Search symbols
mcp_codebase-memory_search_graph \
  --project project-name \
  --query "search term" \
  --label Function

# Get code snippet
mcp_codebase-memory_get_code_snippet \
  --project project-name \
  --qualified_name fully.qualified.SymbolName

# Trace calls
mcp_codebase-memory_trace_path \
  --project project-name \
  --function_name functionName \
  --mode calls \
  --direction inbound
```

### agentic-memory-mcp

```bash
# Add memory event
mcp_agentic-memory-mcp_memory_add \
  --event_type fact \
  --content "memory content"

# Query memories
mcp_agentic-memory-mcp_memory_query \
  --event_types fact \
  --min_confidence 0.7

# Similar search
mcp_agentic-memory-mcp_memory_similar \
  --min_similarity 0.6 \
  --query_text "what I need to know"
```

### obscura

```bash
# Navigate to URL
mcp_obscura_browser_navigate --url https://example.com

# Click element by ref
mcp_obscura_browser_click --ref e3

# Snapshot page
mcp_obscura_browser_snapshot

# Take screenshot
mcp_obscura_browser_take_screenshot --type png
```

### playwright-cdp

```bash
# Navigate
mcp_playwright-cdp_browser_navigate --url https://example.com

# Snapshot
mcp_playwright-cdp_browser_snapshot

# Click
mcp_playwright-cdp_browser_click --target element-selector
```

### deepwiki

```bash
# Ask question about repo
mcp_deepwiki_ask_question \
  --repoName owner/repo \
  --question "How do I use X?"

# Read docs
mcp_deepwiki_read_wiki_contents \
  --repoName owner/repo
```

---

## Common Issues & Fixes

| Issue                               | Fix                                                                             |
| ----------------------------------- | ------------------------------------------------------------------------------- |
| `codebase-memory-mcp` tools failing | Use `codebase-memory` MCP server (without `-mcp` suffix)                        |
| MCP tools not responding            | Run `mcp_codebase-memory_index_repository` to refresh index                     |
| File not found errors               | Check if file is in `.gitignore` — excluded files need direct read              |
| `read_mcp_resource` returns         | `codebase-memory` exposes tools only (0 resources), so it has nothing to read.  |
| "Method not found"                  | Use the `mcp_codebase-memory_*` tools instead. `list_mcp_resources` reports     |
|                                     | which servers do publish resources (currently only `memory`, via `amem://`).    |

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
| Look up API docs, library info               | `deepwiki`           | For GitHub repos and documentation                               |
| Read project OKF knowledge bundle / run BM25 search            | `qmd`               | Use `qmd search/get/ls -c okf-mcp`; no LLM model needed            |

**Workflow:**

1. Before editing code → `codebase-memory` to find all references and callers
2. Before testing changes → `obscura` to verify UI behavior
3. After completing task → `vestige-mcp` to store session context
4. When stuck on API → `deepwiki` to check documentation
5. Before/after any task → consult `qmd` (`qmd search/get -c okf-mcp`) for the OKF bundle; persist findings to `okf-mcp/` (qmd-aware) and session memory

---

## Project

**goIsekai** is a manga library manager and reader: an embedded HTTP server + browser UI. Plugins (Lua, JS, Go/wasm, Yaegi, native) fetch manga from external sites. The host owns libraries, reading progress, image caching and enrichment.

Module `goisekai` · Go 1.27.1 · **CGO-free** (pure Go SQLite) · server entry `cmd/goisekai/main.go` · binary `./goisekai`, listens `127.0.0.1:8080`.

Repo layout: `cmd/` (server, tray, jetgen), `internal/` (all app code), `pkg/types` (plugin ABI), `examples/` (canonical plugin + info sources), `tools/build-wasm.sh`, `docs/`, `openspec/`.

---

## Commands

Recipes live in the `justfile` (`just --list`). **Prefix git/npm/etc. with `rtk`** for compact output (see `CLAUDE.md`).

| Command          | What it does                                                               |
| ---------------- | -------------------------------------------------------------------------- |
| `just build`     | `css` + `br` + `CGO_ENABLED=0 go build -o goisekai ./cmd/goisekai`          |
| `just test`      | `CGO_ENABLED=0 go test ./internal/... ./pkg/... ./cmd/...`                  |
| `just race`      | same tests with `-race` (`CGO_ENABLED=1`)                                   |
| `just check`     | Production-Go gate: fmt + modernize + lint (tests/Lua/web elsewhere)        |
| `just lint` / `just lint-prod`      | golangci-lint (all / prod only)              |
| `just modernize` | `modernize -fix` for the go.mod Go version                                  |
| `just run *args` | build then `./goisekai -logLevel debug`; `just run -port 9099` passes args  |
| `just dev`/`devrun` | aliases of `build` / `run`                                              |
| `just fmt` / `fmt-prod` / `fmt-lua` / `fmt-web` | go fmt / go fmt prod / stylua / biome -write |
| `just lint-lua` / `lint-web`        | luacheck / biome (read-only)                                 |
| `just br`        | **regenerate `*.js.br` / `*.css.br` — mandatory after any JS/CSS edit**     |
| `just css`       | compile `cmd/goisekai/frontend/lib/tailwind.css` (needs npx)                |
| `just open`      | open the browser at the server URL                                          |
| `just install-lua <x>` / `install-js <x>` / `install-info <x>` | copy `examples/…/<x>` into `app_data/plugins` or `app_data/info` |
| `just clean`     | remove the server binary                                                    |

**No install recipe for wasm/native plugins** — copy manually: `cp examples/plugins/wasm/<x>/main.wasm app_data/plugins/<x>/` and `cp examples/plugins/native/<x>/main.so app_data/plugins/<x>/`.

Raw equivalent for a quick rebuild + restart:

```sh
CGO_ENABLED=0 go build -o goisekai ./cmd/goisekai
pkill -x goisekai; sleep 2
(setsid nohup ./goisekai -logLevel debug > goisekai.log 2>&1 &); sleep 8
curl -s -o /dev/null -w '%{http_code}\n' http://localhost:8080/   # expect 200
```

Restarting without `pkill` first fails with `bind: address already in use`.

---

## Architecture

```
cmd/goisekai/main.go          ← wiring: config → db → proxy → plugin manager → server
cmd/goisekai/maintenance.go   ← startup pruning, backups, periodic jobs
internal/bridge/service.go    ← ApplicationService — glues DB, plugin manager, proxy into one API
internal/httpserver/*         ← routes.go, views (Lua templates), API handlers, actions, static_br.go
internal/database/*           ← SQLite (modernc.org/sqlite), go-jet DSL (.gen/), schema.go migrations
internal/pluginmanager/*      ← plugin discovery/loading: lua (lunar), js (goja), wasm, yaegi, native .so (purego)
internal/workers/*            ← job pool: LaneInteractive / LaneFetch / LaneImage / LaneMaintenance / LaneCDP
internal/hostnet/*            ← HTTP proxy + CDP browser engine for anti-bot solving
internal/logger/*             ← slog-based logger; level via -logLevel / goisekai.ini hot reload
internal/enrich/*             ← enrichment registry (providers come from info scripts)
internal/templates/*          ← Lua templates for HTML rendering
internal/config/*             ← hand-rolled INI parser (goisekai.ini)
internal/bridge/cache,image   ← image caching + fetch pipeline (singleflight, host lanes, pacing)
pkg/types/*                   ← plugin ABI contract, shared types
examples/plugins/{lua,js,wasm,yaegi,native}  ← canonical plugin sources
examples/info/*               ← canonical info scripts
```

**Search flow:** handler → `AppService.Search()` → `PluginManager.Search()` → instantiate VM → `Search()` export → JSON → return.

**Read flow:** reader → chapters from DB → `GetPageList()` → pages cached on disk → served through `/image` proxy.

**Plugin ABI** (`pkg/types/abi.go`): export `Search`, `GetMangaDetail`, `GetChapterList`, `GetPageList` (JSON string over string). Optional: `Init`, `GetGenres`, `GetEnrichment`. All network goes through the host (`host_http_request` / native `host_call`) — plugins do no I/O themselves. Chapter objects emit `id`, `manga_id`, `title`, `chapter_num`, optionally `released_at`.

**Info scripts** are metadata-only Lua in `app_data/info/<source>/main.lua`, discovered separately and hidden from the plugin list. They declare `PLUGIN.enrichment_providers` and implement only `getEnrichment`.

---

## Database

- SQLite via `modernc.org/sqlite` (pure Go); WAL + busy timeout 5000ms.
- DB: `<data_dir>/goisekai.db` (default `app_data/goisekai.db`).
- **Always go-jet DSL** in `internal/database/.gen/` — never hand-write SQL outside migrations.

```go
Chapters.INSERT(Chapters.ID, Chapters.MangaID, ...).Exec(d.db)
SELECT(Chapters.Title).FROM(Chapters).WHERE(Chapters.ID.EQ(String(id))).Query(d.db, &out)
```

Dot-import `. "github.com/go-jet/jet/v2/sqlite"` in `*_query.go` is intentional, excluded from staticcheck.

- Tables: `mangas`, `chapters`, `chapter_pages`, `plugins`, `read_history`, `alt_titles`, `alt_covers`, `alt_descriptions`, `manga_categories`, `manga_related`, `plugin_cache`, `plugin_verify`, FTS5 `library_fts`.
- `plugin_cache` (sqlite, `UNIQUE(plugin_id, manga_id, function_name)`) caches plugin responses — **after changing plugin output clear it**: `DELETE FROM plugin_cache WHERE plugin_id='<id>';` or the old payload keeps being served.
- Migrations: `internal/database/schema.go` `migrations` slice, applied by `runMigrations()` via `PRAGMA user_version`.

---

## Plugin System

Five kinds:

| Kind      | Runtime          | Entry      | Notes                                        |
| --------- | ---------------- | ---------- | -------------------------------------------- |
| `lua`     | lunar VM         | `main.lua` | full Lua 5.4, host functions injected        |
| `js`      | goja VM          | `main.js`  | ES5-compatible, host functions injected      |
| `wasm`    | wasm runtime     | `main.wasm`| built by `tools/build-wasm.sh` (TinyGo → std-Go wasip1 fallback) |
| `native`  | purego `dlopen`  | `main.so`  | C/C++ plugin, built by `examples/…/build.sh` |
| `yaegi`   | Yaegi            | `.go` files| interpreted Go dialect                       |

- Source of truth lives in `examples/`; `app_data/` is **gitignored**, so installed plugins are never tracked. `just install-lua|js|info` copies them in.
- **Native `.so` is mapped from a PRIVATE copy** (`goplug_native.go`): `dlopen` keeps the file mmap'd, so an in-place reinstall (`cp`, `os.Create`) would truncate the same inode → SIGSEGV in the host. Do not "simplify" that copy away.
- Native `.so` links with `strip` at build time (smaller); `*.so` is gitignored, `main.wasm` **is** tracked in `examples/`.
- Lazy loading: first call instantiates the VM; one `sync.Mutex` per plugin; **15s invoke timeout** per call.
- Network goes through `hostnet.Proxy` (TLS fingerprinting, CDP challenge solving, default headers).
- Reload a plugin live: `POST /api/sandbox/plugins/<id>/reload`.
- Per-plugin dev endpoints: `/api/sandbox/plugins/{search,detail/{id},chapters/{id},pages/{id}}` — **there is no `genres` route there** (404 is expected); genres load through `/view/search` → `AppService.ListGenres`.

---

## Frontend

- **Templates**: Lua + HTML in `internal/templates/` (`layouts/`, `views/`, `partials/`); `views/*.lua` = full page, `partials/*.lua` = required sub-templates.
- **Engine**: `internal/templates/lua_engine.go` (`h()` escapes HTML).
- **JS**: Alpine.js + `cmd/goisekai/frontend/lib/alpine-components.js`; **Biome** for formatting (not ESLint/Prettier).
- **Styles**: Tailwind 3.4 → `frontend/lib/tailwind.css` via `just css`.
- **Static serving is from disk, not `go:embed`**: `frontend_dir` from `goisekai.ini` → `http.FS` → `brHandler` in `static_br.go`, which serves the pre-compressed `.br` sibling when the client accepts brotli, with `Cache-Control: no-cache` (asset URLs are unversioned, so a stale copy would survive a rebuild).
- **`.br` gate**: any edit under `frontend/lib/*.js|*.css` requires `just br`, otherwise `TestBrotliSiblingsMatchSource` (in `internal/httpserver/frontend_assets_guard_test.go`) fails and the served bundle is the stale one.
- **SPA routing**: `X-Partial: true` returns only `<main>` (no layout wrapper).

---

## Config

- File `goisekai.ini` (auto-generated) or `$GOISEKAI_CONFIG`; hand-rolled INI.
- Sections `[app]`, `[network]`, `[maintenance]`. Keys: `data_dir`, `cache_dir`, `frontend_dir`, `templates_dir`, `info_dir`, `image_format`, `cover_max_dim`, `host`, `port`, `referer`, `log_level`.
- CLI overrides config: `-logLevel`, `-host`, `-port`, `-cdpEngine`, `-cdpPath`, `-apiKey`.
- **Hot reload** every 5s: safe fields (log level, user-agent, referer) apply live.

---

## Testing

- `*_test.go` next to the code; fixtures in `internal/pluginmanager/testdata/`.
- Run `just test` / `just race`.
- **Known flaky**: `TestSamePriorityFIFO` (`internal/bridge/image_test.go`) — timing-sensitive. When the bridge suite needs to be green, run `go test ./internal/bridge/ -skip TestSamePriorityFIFO`.
- CDP end-to-end: `internal/hostnet/cdp_e2e_test.go` (needs a browser).
- Plugin wasm/native verification is live-only: build, copy to `app_data/plugins/<id>/`, `POST /api/sandbox/plugins/<id>/reload`, then hit the sandbox endpoints. Clear `plugin_cache` if old output lingers.

---

## Gotchas & Conventions

- **Always `CGO_ENABLED=0`** (pure Go SQLite) except `just race`.
- **Plugin files**: `app_data/plugins/<id>/` at runtime — Lua/JS are directories; wasm is `main.wasm`; native is `main.so`. The web asset route uses a **hyphen**: `/plugin-static/{pluginID}/{file}`.
- **`.br` regeneration** after any frontend JS/CSS edit (see Frontend).
- **Server restart discipline**: `pkill -x goisekai; sleep 2` before start, else port 8080 bind conflict.
- **Image cache**: `<cache_dir>/images/`; pages keyed by a deterministic hash. Cover pacing applies only to MangaDex at-home hosts (`*.mangadex.network`); the 2-high/1-low admission cap applies to every host.
- **PID file** `<data_dir>/goisekai.pid` (removed on shutdown); orphan pruning + periodic backups to `<data_dir>/backups/`.
- **Logs**: every `print*` must go through `internal/logger` (no stdout prints); job/lifecycle records carry a structured `lane=` attribute the logs view filters on.
- **gitignore**: `app_data/`, `*.so`, `*.har`, `*.log`, `goisekai`, `tmp/`, `.slim/`, `.rtk/`, `.codebase-memory/`, `CLAUDE.md`. `openapi*` is NOT ignored. Large HAR captures live in `tmp/`.
- **Biome** (frontend), **Stylua** + **Luacheck** (templates: `just lint-lua`).

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

### qmd (OKF knowledge bundle — local GGUF search)

```bash
# Init per-project (one-time; creates .qmd/index.yml)
qmd init

# Index the bundled OKF memories (no collection path arg uses last index.yml)
qmd update

# BM25 keyword search — no LLM model required, fast & local
qmd search "LaneCDP anti-bot workers pool" -c okf-mcp
qmd search "image flight dedupe" -c okf-mcp

# List/indexed documents
qmd ls okf-mcp
qmd get qmd://okf-mcp/task/workers-pool-refactoring-and-migration-to-pond-v2.md

# Hybrid/semantic (OPTIONAL — needs `qmd pull` to download ~2GB GGUF models)
qmd embed        # builds vector index
qmd query "how does per-plugin CDP timeout route"  # LLM expansion + rerank
```

`qmd` ships on NVM (local), uses **local GGUF/HF models** (embeddinggemma, query-expansion, Qwen reranker) — **not** Ollama-dependent by default. For exploring the `okf-mcp/` bundle, `qmd search` (BM25) is sufficient and needs **no** model download.

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
| Frontend fix "not working" after edit | `just br` then rebuild + restart the server (assets are read from disk)       |
| Plugin returns stale/old output     | `DELETE FROM plugin_cache WHERE plugin_id='<id>';` + `POST /api/sandbox/plugins/<id>/reload` |
| `bind: address already in use`      | `pkill -x goisekai; sleep 2` before starting                                    |
| Bridge suite red on `TestSamePriorityFIFO` | timing flake — rerun with `-skip TestSamePriorityFIFO`                    |

## Notes

- (add quick facts here)

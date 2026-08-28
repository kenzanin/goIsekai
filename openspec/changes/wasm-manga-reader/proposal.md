## Why

We need a cross-platform desktop manga reader whose content sources are hot-swappable without recompiling the host. Manga sources differ wildly in their HTML/JSON shape and often require custom headers, referers, and session state to bypass hotlink/CORS protections — so a native, plugin-based architecture with a sandboxed network proxy is the right fit. Building this on Go + WASM keeps the entire stack CGO-free so a single static binary ships to Windows, Linux, and macOS.

## What Changes

- Introduce a shared, versioned data contract (DTOs) for manga, chapters, pages, and search filters.
- Define the plugin interface: four JSON-over-memory host functions every source plugin must export (`Search`, `GetMangaDetail`, `GetChapterList`, `GetPageList`).
- Introduce a sandboxed host network proxy: plugins may not open sockets directly and must route all HTTP through a host-imported `host_http_request`, with automatic header injection and per-source cookie persistence.
- Introduce a Pure-Go SQLite persistence layer (library, chapters, read history, installed plugins) with a fixed DDL schema.
- Introduce a WASM plugin runtime (wazero/extism) with discovery, resource limits (memory cap, per-call timeout), and panic isolation.
- Introduce the Wails frontend bridge: `AppService` bindings plus a custom `manga-img://` asset proxy that serves image bytes with correct referers to eliminate CORS/hotlink failures.

## Capabilities

### New Capabilities
- `plugin-abi`: The versioned DTO contract and the host-function interface every source plugin must implement.
- `host-network`: Sandboxed HTTP execution — host-side request handling, header/session enforcement, and cookie persistence.
- `storage`: SQLite persistence for library bookmarks, chapter metadata/read progress, download status, and installed plugins.
- `plugin-runtime`: WASM plugin lifecycle — discovery, loading, resource limits, timeout enforcement, and panic recovery.
- `bridge`: Wails service bindings exposed to the frontend and the local image proxy protocol.

### Modified Capabilities
<!-- none - greenfield project -->

## Impact

- **New Go module** (`go.mod`, Go 1.21+) with packages: `pkg/types`, `pkg/hostnet`, `pkg/database`, `pkg/pluginmanager`, `pkg/bridge`.
- **New dependencies**: Wails (v2/v3), `wazero` and/or `extism` (pure-Go WASM runtime), `glebarez/go-sqlite` or `modernc.org/sqlite` (pure-Go SQLite).
- **Build constraint**: `CGO_ENABLED=0` across all host builds (Windows, Linux, macOS).
- **Deployment artifact**: a self-contained desktop app plus an `app_data/plugins/*.wasm` discovery directory.
- **No existing code affected** (greenfield repository).

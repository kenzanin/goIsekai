## 1. Project Setup

- [ ] 1.1 Initialize the Go module (`go mod init`) with Go 1.21+ and add `wazero` and `modernc.org/sqlite` dependencies
- [ ] 1.2 Add Wails scaffolding (v2/v3) and confirm a CGO-free host build config (`CGO_ENABLED=0`) for all target platforms
- [ ] 1.3 Establish the standard Go project layout: `cmd/goisekai` (entrypoint), `pkg/types` (shared ABI contract), `internal/{hostnet,database,pluginmanager,bridge}` (host-private packages)

## 2. Shared ABI & Core Domain Types (`pkg/types`)

- [ ] 2.1 Define the `Manga`, `Chapter`, `Page`, and `SearchFilter` DTOs with JSON tags matching the proposal contract
- [ ] 2.2 Define the plugin interface spec: the four exported host functions (`Search`, `GetMangaDetail`, `GetChapterList`, `GetPageList`) and the `host_http_request` import signature
- [ ] 2.3 Add the `contractVersion` constant and the version-declaration/rejection mechanism

## 3. Host Network Proxy (`internal/hostnet`)

- [ ] 3.1 Implement the `host_http_request` handler that decodes the request payload, performs the HTTP call, and returns the response as JSON
- [ ] 3.2 Implement automatic standard-header injection (`User-Agent`, `Accept-Language`, configured `Referer`)
- [ ] 3.3 Implement per-page header override (page `headers` map wins over defaults)
- [ ] 3.4 Implement per-plugin cookie jars (one `cookiejar.Jar` keyed by plugin id) with persistence across calls

## 4. SQLite Schema & Persistence (`internal/database`)

- [ ] 4.1 Set up the pure-Go SQLite connection and a versioned migration runner
- [ ] 4.2 Create the `mangas` table with the `UNIQUE(plugin_id, source_manga_id)` constraint
- [ ] 4.3 Create the `chapters` table with `is_read`, `last_page_read`, `download_status`, and FK cascade to `mangas`
- [ ] 4.4 Create the `read_history` and `plugins` tables with cascade and registry fields per the DDL
- [ ] 4.5 Implement repository methods for library bookmarks, chapter read progress, download status transitions, read history, and plugin registration

## 5. WASM Engine & Isolation (`internal/pluginmanager`)

- [ ] 5.1 Implement plugin discovery by scanning `app_data/plugins/*.wasm`
- [ ] 5.2 Implement module loading into the `wazero` runtime with the four JSON host functions bound to each plugin
- [ ] 5.3 Wire `host_http_request` as the host-imported function available to plugin modules
- [ ] 5.4 Enforce the 64 MB per-instance memory cap and 15s per-invocation timeout (context cancellation)
- [ ] 5.5 Implement panic/OOM interception so plugin failures return a Go `error` and leave the host/UI functional

## 6. Wails Frontend Bridge & Asset Proxy (`internal/bridge`)

- [ ] 6.1 Implement `SearchManga`, `GetMangaDetails`, `GetPageList` service bindings that delegate to plugins
- [ ] 6.2 Implement `ToggleLibraryItem` and `InstallPlugin` service bindings backed by `internal/database` and `internal/pluginmanager`
- [ ] 6.3 Register the `manga-img://` protocol handler that fetches image bytes with the page's headers (including `Referer`) and serves them to the frontend
- [ ] 6.4 Add image caching so repeat requests satisfy the sub-second cached-image latency requirement

## 7. Acceptance Verification

- [ ] 7.1 Verify `CGO_ENABLED=0` cross-compilation succeeds for Windows, Linux, and macOS
- [ ] 7.2 Verify a panicking/OOM `.wasm` plugin returns a Go error and the UI remains functional
- [ ] 7.3 Verify manga progress, offline download status, and library bookmarks persist accurately across restarts
- [ ] 7.4 Verify cached image loading via `manga-img://` stays under one second

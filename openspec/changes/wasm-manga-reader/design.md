## Context

Greenfield Go repository (no `go.mod`, no existing code) targeting a cross-platform desktop manga reader. The stack is fixed by the proposal: Go 1.21+, Wails (v2/v3), a pure-Go WASM runtime (`wazero`/`extism`), and a pure-Go SQLite driver. The hard constraint is `CGO_ENABLED=0` for all host builds so the runtime and DB layers must have no cgo. Plugins are Go compiled to WASM (`wasip1/wasm`), never run in-process, and get network access only through a host-imported function.

## Goals / Non-Goals

**Goals:**
- A working host that discovers `.wasm` source plugins, sandboxes them, and routes their HTTP through the host.
- A single, versioned ABI so the host and plugins can be versioned independently.
- CGO-free builds on Windows, Linux, and macOS.
- Panic/OOM isolation so a broken plugin returns a Go `error` and never takes down the UI.

**Non-Goals:**
- The offline chapter download *pipeline* itself (the `download_status` field and its transitions are in scope; background download orchestration, dedup, and disk layout are a later phase).
- A plugin SDK/source-distribution mechanism (the ABI is the contract; authoring tooling comes later).
- Full anti-bot bypass (TLS fingerprinting beyond basic header/transport tuning is a stretch goal, not a gate).

## Decisions

### D1: Use `wazero` as the WASM runtime (over `extism`)
The plugin surface is four JSON-in/JSON-out functions plus one host import (`host_http_request`). `wazero` exposes this directly with fine control over host-imported functions, per-module memory limits (`wazero.NewRuntimeConfig().WithMemoryLimitPages(...)`), and context-based cancellation for the per-call timeout. `extism` adds a PDK layer and its own host-function model that is useful when distributing cross-language plugins, but here the only plugin author is Go and the ABI is already JSON-over-memory — `extism`'s abstraction buys nothing and obscures the resource-limit hooks.

*Alternative considered:* `extism` for cross-language plugin authoring — rejected because it couples the ABI to the PDK's serialization and complicates the exact `host_http_request` shape we already own.

### D2: Use `modernc.org/sqlite` as the SQLite driver
Pure Go, no cgo, well-maintained, and a drop-in `database/sql` driver. The `glebarez/go-sqlite` alternative is also pure Go but adds a translation layer over `modernc` for marginal ergonomic benefit.

*Alternative considered:* `glebarez/go-sqlite` — rejected to depend directly on the upstream implementation.

### D3: Compile plugins to `wasip1/wasm` (not `js/wasm`)
`wasip1` targets a clean WASI-style import ABI with no JS host assumptions, which maps cleanly onto `wazero`'s host module. `js/wasm` assumes a JS environment we don't have inside the host.

### D4: Version the contract explicitly
The ABI carries a `contractVersion` integer. The host exposes it via a host function; each plugin declares the version it was compiled against via an exported symbol. The host rejects a plugin whose declared version mismatches the host's. This prevents silent mis-parsing when the DTO shapes evolve.

*Alternative considered:* no versioning (rely on JSON being tolerant) — rejected because a missing/renamed field silently corrupts data; the proposal's DTOs are a compatibility boundary.

### D5: One cookie jar per plugin, keyed by plugin id
Session/cookie state is scoped to the source, not shared globally. The host holds a `cookiejar.Jar` per plugin and attaches it to the transport used for that plugin's requests, so authenticated sources keep working across calls while different sources stay isolated.

### D6: Image proxy via Wails asset/custom-scheme handler
Register a custom scheme (`manga-img://`) handled in-process. The handler parses the encoded page URL + headers, performs the fetch with the page's `Referer`/`User-Agent`, and streams bytes back. This sidesteps CORS and hotlink referer checks in the webview. (True OS-level scheme registration is platform-specific; the exact Wails mechanism — asset handler vs native scheme — is confirmed during Wails v2/v3 wiring, see Open Questions.)

## Risks / Trade-offs

- **Pure-Go SQLite is slower than cgo SQLite** → Acceptable for a reader's small writes; mitigations if needed: batched transactions and prepared statements.
- **WASM call overhead per request** → JSON-over-memory serialization is cheap relative to network I/O; keep payloads bounded and reuse module instances where possible.
- **TLS fingerprinting for bypass sources is hard to make robust** → Scope it as best-effort header/transport tuning first; full uTLS-style fingerprinting is deferred (see Open Questions). Broken bypass degrades to "source unavailable", never crashes the host.
- **Custom image scheme may differ between Wails v2 and v3** → Isolate the proxy behind an interface in `pkg/bridge` so the scheme registration is the only platform-specific piece.
- **Memory cap too low for image-heavy plugins** → 64 MB is a starting point; expose the cap as a per-plugin configurable value, keep 64 MB as the default.

## Migration Plan

Greenfield — no data or code to migrate. Establish the SQLite schema via versioned migrations from day one (`migrate`-style numbered steps) so future schema changes are incremental. Rollback for the initial release is simply "don't ship"; no in-place upgrade path exists yet.

## Open Questions

- **Wails v2 vs v3 and the image-scheme mechanism:** both support serving bytes to the webview, but the exact API for a custom scheme differs. This does not change the specs (the `manga-img://` contract is fixed) — resolve during bridge implementation.
- **TLS fingerprinting library (e.g. `utls`) vs raw `net/http`:** affects bypass reliability for Cloudflare-protected sources, not the ABI or schema. Resolve when a concrete bypass source is targeted.
- **`wazero` memory cap default:** 64 MB is assumed; tune per source after profiling real plugins.

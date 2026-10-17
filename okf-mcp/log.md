# Bundle Update Log

## 2026-10-11

- Bundle initialized with `pi-okf init`.
- Added concept/cover-refresh-vs-clear-actions.md: Refetch re-downloads the main cover (busts caches), Clear drops the alt_covers candidate list (toast `Cover candidates cleared`, evidence cover_clear_refresh.har).
- Converted EchoesVault knowledge pages to concepts:
  - concept/enhance-status-log.md: startup image-enhance status log (formatEnhanceStatus/LogEnhanceStatus wiring, log format examples, edge cases).
  - concept/reader-loading-cancel-fix.md: reader loading cancel via AbortController (reader.js abortLoad, bars visible during load, reader.lua spinner display:none).
  - concept/library-fts-index-sync.md: FTS5 CAST key-comparison fix, 7 self-syncing mutation choke points, LibraryFTSDrift/EnsureLibraryFTS auto-heal, 10k-row INTEGER-key ceiling.
  - concept/wasm-plugin-local-helpers-decision.md: DECISION keep local helpers on wasm hot paths — hostCall IPC+JSON roundtrip cost, status map is plugin data, coregex/go-json size tradeoff.
- Consolidated 8 EchoesVault daily scratchpads into concept/session-dailies-2026-08-29-to-09-30.md (tls-client migration, human-verify/WebP ship, image burst pacing root cause, tailwind path bug, wasm host helpers + plugin ports, decimal-chapter audit, FanFox findings).

## 2026-10-17

- Added concept/wasm-cpp-plugin-toolchain.md: WASM plugins can be written in C++17 and built with the system wasi-sdk clang (`--target=wasm32-wasi -mexec-model=reactor`, RapidJSON, `-idirafter /usr/include`). Covers the `libxml2.so.2` shim for wasm-ld, the duplicate-export trap (`_initialize`/`malloc`/`free` come from the reactor CRT), the three reusable headers (`wasm_abi.h`, `util.h`, `html_util.h`) shared by `komix_to_cpp` and `manhawato_cpp`, host-helper delegation via `host_str` (`text.url_encode`/`url_decode`/`date_to_iso`/`strip_html`/`unescape`), and the measured size comparison: C++ ~0.3 MB vs TinyGo ~1.9 MB vs standard Go 3.9-5.2 MB. Shipped commit a628bd5; `manhawato_cpp` replaces the Go wasm in `app_data/plugins/manhawato/`.


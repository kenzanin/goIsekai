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


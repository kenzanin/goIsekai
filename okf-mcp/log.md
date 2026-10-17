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

- Added concept/reader-two-page-view.md: the reader's two-page layouts for sources that split one original page into two images. `spread` is a tri-state (`''`/`side`/`stack`), cycled with every other read mode by one A/B/C/D button (`#btn-readmode`; A 1 page · B 2 stacked · C 2 side by side · D continuous strip) — `side` puts the halves left/right (rtl reverses order), `stack` puts them one above the other so they read as one continuous page. Records the contracts that keep it working (`stepSize()` = 2 per step, trailing odd page stands alone, strip mode wins, counter shows a range, `viewBox()` scales the pair as one unit), the measured canvas geometry that tells the two layouts apart (`side` 2083x1600 vs `stack` 1125x2720 for pages 958x1120 + 1125x1600), the `img2` resets, and the opaque-JSON per-manga `spread` pref.
- Added concept/library-duplicate-colour-bands.md: the library grid flags duplicate groups with a 3px left colour band, one colour per group (`dupPalette` in `views_library.go`, indexed by group position → `DupColors`). Records why the band is an inline `style` and not a Tailwind class (Tailwind can only generate classes it can see spelled out; `"border-" .. color` renders nothing), why `border-left` over `ring`/full `border` on a `rounded-lg overflow-hidden` card, the palette rules (readable on `bg-neutral-900`, must avoid the indigo accent), the new `?sort=duplicates` option, and the determinism the colours depend on: `FindPotentialDuplicates` now sorts equal-size groups by `Key` (it previously used a bare `sort.Slice` with no tiebreak, so bands could recolour between restarts). Verified by restarting twice and diffing the band colours.
 and built with the system wasi-sdk clang (`--target=wasm32-wasi -mexec-model=reactor`, RapidJSON, `-idirafter /usr/include`). Covers the `libxml2.so.2` shim for wasm-ld, the duplicate-export trap (`_initialize`/`malloc`/`free` come from the reactor CRT), the three reusable headers (`wasm_abi.h`, `util.h`, `html_util.h`) shared by `komix_to_cpp` and `manhawato_cpp`, host-helper delegation via `host_str` (`text.url_encode`/`url_decode`/`date_to_iso`/`strip_html`/`unescape`), and the measured size comparison: C++ ~0.3 MB vs TinyGo ~1.9 MB vs standard Go 3.9-5.2 MB. Shipped commit a628bd5; `manhawato_cpp` replaces the Go wasm in `app_data/plugins/manhawato/`.


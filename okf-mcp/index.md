---
okf_version: "0.2"
---

# .

Progressive disclosure index for this OKF bundle.

## Concepts

* [Example concept](concept.md) - Sample concept document with full frontmatter
* [Cover refresh vs clear actions](concept/cover-refresh-vs-clear-actions.md) - What Refetch and Clear do on the manga detail cover menu
* [Enhance status startup log](concept/enhance-status-log.md) - Startup log of effective image-enhance status per plugin, with format examples and edge cases
* [Reader loading cancel fix](concept/reader-loading-cancel-fix.md) - AbortController-based cancel during reader loading; nav bar visible, spinner flash prevented
* [library_fts index sync and auto-heal](concept/library-fts-index-sync.md) - FTS5 sync choke points, CAST key-comparison fix, drift-check auto-heal, scalability ceiling
* [Wasm plugin local helpers decision](concept/wasm-plugin-local-helpers-decision.md) - Why wasm hot paths keep local helpers instead of per-row hostCall
* [Odin WASM plugin port candidates](concept/odin-wasm-plugin-port-candidates.md) - 14 built plugins (mangaread added 2026-10-20) with live A/B, corrected migration tiers (gmatch is not the blocker, kaliscan dead), the Lua-regex/RE2, plugin_cache, dangling-builder, unexported-JSON, find_all-vs-find_all_pairs, frozen-WASI-clock (2022-01-01), Madara-ajax-chapters, mangakatana-JS-array and per-card-search traps, the abi/ host-helper normalization, and the 7 C++ twins now installed (14.5 MB → 1.35 MB)
* [Host helper surface registry](concept/host-helper-surface-registry.md) - One Go table now defines every plugin-callable helper; lua/js/wasm read it, closing the JS text.trim/unescape/lua_escape drift
* [C++ WASM plugin toolchain (wasi-sdk) and binary-size comparison](concept/wasm-cpp-plugin-toolchain.md) - wasi-sdk clang++ reactor build, RapidJSON, shared headers, host-helper migration, and C++/TinyGo/standard-Go size measurements
* [Session dailies 2026-08-29 to 2026-09-30](concept/session-dailies-2026-08-29-to-09-30.md) - Roll-up of 8 daily scratchpads: tls-client migration, image burst pacing root cause, wasm ports, decimal-chapter audit, FanFox findings
* [Onisaga genre support findings](concept/onisaga-genre-support-findings.md) - Host genre contract, onisaga page structure, and the open slug-vocabulary mapping for search-genre support
* [Onisaga Cloudflare challenge behavior](concept/onisaga-cloudflare-challenge.md) - Observed Cloudflare blocking on onisaga fetches and the cookie/TLS evidence so far
* [VioletManga search/detail selector findings](concept/violetmanga-search-detail-findings.md) - Bixbox theme selectors, bare-slug IDs, pure-HTTP pages
* [Zeroscann search/detail selector findings](concept/zeroscann-search-detail-findings.md) - Vuetify card scoping, slash chapter hrefs, Referer-gated images
* [Reader two-page view](concept/reader-two-page-view.md) - spread layouts (`side`/`stack`) for pages split into two images, tri-state toggle, step/viewBox contracts, measured render geometry
* [Library duplicate-group colour bands](concept/library-duplicate-colour-bands.md) - per-group left colour band on library cards, palette rules, and the group-order determinism the colours depend on
* [Dead code and merge candidates audit](concept/dead-code-and-merge-audit.md) - 19 unreachable functions, the unused migrate_candidates.go, the half-wired workers.Permanent() bug, and six verified merge candidates
* [Detail-page read toggle, genre tags, and reader-prefs persistence](concept/detail-page-read-genre-readerprefs-fixes.md) - Three bugs that looked frontend-only: the derived Done flag vs toggle-read, cachedMangaFallback dropping genres, and two reader.js faults (no unload flush, stale server zoom snapshot)
* [Plugin runtime export matrix (lua vs js vs yaegi vs go vs wasm)](concept/plugin-runtime-export-matrix.md) - Per-runtime ABI export names, meta source, contract-version enforcement, host-helper surface, and six inconsistencies worth normalizing (why wasm has no `Init`)
* [Test coverage gaps in internal/pluginmanager](concept/pluginmanager-test-coverage-gaps.md) - What the 60.0% -> 70.7% coverage pass still leaves unreached, why `loadGo`/`callGo` are 0% (the `.so` kind has never loaded), and the two known live defects tests do not catch (`comix` returns 0 results, `TestSamePriorityFIFO` flake)
* [Native .so plugin ABI (purego loader)](concept/native-so-plugin-abi.md) - Loading C/C++ `.so` plugins without cgo: `char*(const char*)` JSON ABI, plugin-owns-the-buffer + required `plugin_free`, the `{"error"}` envelope, and why stdlib `plugin` could never work in a `CGO_ENABLED=0` build
* [Home reload NS_BINDING_ABORTED (saved-filter redirect)](concept/home-reload-ns-binding-aborted.md) - Why Firefox cancels plugin logo/favicon requests on a library reload (the client-side `location.replace()` that restores the saved filter), why a manual request still returns 200, and the unfixed fix options

* [Chapter bound with chapter_num 0](concept/chapter-bound-zero-chapter-num.md) - Why bulk "set this and before/after as read" marked the whole manga: chapter_num alone is not a key, (chapter_num, id) lexicographic bound fixes comizy-style all-zero sources
* [Info scripts as Odin wasm](concept/info-scripts-as-odin-wasm.md) - Enrichment scripts run as wasm: the helper surface was already complete, discovery's hardcoded main.lua was the real blocker, required exports switch on the info: prefix, and a Lua-only title-matching bug the port fixed
* [Weebcentral search cover alignment](concept/weebcentral-search-card-cover-alignment.md) - Every search card renders TWO img (desktop + mobile variant), so a positional zip put each cover on the previous title; match covers to cards by the ULID the cover URL carries, never by position
* [Wasm install deadlock](concept/wasm-install-deadlock.md) - Install held the manager lock and loadWasm -> ensureWasmRuntime took it again; sync.RWMutex is not reentrant, so installing a .wasm hung the whole host. Runtime now has its own lock
* [Chapter release date column](concept/chapter-release-date-column.md) - fetched_at was overwritten with CURRENT_TIMESTAMP and then read back as the release date, so every chapter showed the sync day; chapters now has a dedicated released_at column (NULL when the plugin knows no date), plus the jetgen DDL drift that made regenerating .gen delete a real column
* [Chapter order newest-first (ABI)](concept/chapter-order-newest-first-abi.md) - Two plugins reversed the site's newest-first chapter fragment, so a library manga read 74,73,72… and the same series live read 1,2,3…; the ABI is newest-first, comizy's "progress is keyed off chapter_num" was false, and split labels "Chapter 12-2" need folding to 12.2
* [Prod file split refactor](concept/prod-file-split-refactor.md) - Every prod file over 200 lines that held many functions is now split (17 files, 32 new ones); the rule for when to leave a file alone, the two traps when cutting Go blocks by hand, and the deadcode/staticcheck results
* [Native .so plugins build with zig c++](concept/native-zig-build.md) - New tools/build-native.sh builds every native plugin with zig c++ (g++ fallback): static libc++ means libc.so.6 only, at ~2x the size; the per-plugin -fvisibility=hidden flag is a contract, not a global
* [Static musl .so](concept/static-musl-so.md) - A shared object can be fully self-contained: zig c++ against the host's PIC musl libc.a gives a .so with zero DT_NEEDED entries, so the host needs no libc at all; plus the three traps (--version-script and -Bsymbolic both segfault, -I/usr/include breaks libc++)
* [Native 1manga plugin](concept/native-1manga-plugin.md) - 1manga now runs as a native .so (the Lua original stays in the archive as the reference); byte-identical to the Lua build on all four ABI calls, and the trap that broke it: RapidJSON writes {"index":0.0} where Go wants an integer

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


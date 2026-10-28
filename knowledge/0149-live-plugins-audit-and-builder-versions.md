---
type: audit
title: "Live plugins audit: builder/runner versions and released_at parsing status"
description: "Audit of all 43 installed (live) plugins under app_data/plugins/. Records builder/runner versions (Odin dev-2026-10:84bc3fc21, TinyGo 0.43.0/LLVM 22.1.4, Go 1.26.9 pinned, Lua/JS/yaegi host-runtime) and per-plugin released_at parsing status (host ABI field pkg/types/types.go:39 ReleasedAt; there is NO created_at). Findings: mangaread_odin ALREADY parses released_at and live wasm contains the parser (the 'unreachable' live behaviour is a network/human-verification issue, not a missing-field bug); mangapill intentionally omits released_at (source HTML publishes no per-chapter date); 21 other plugins do not parse dates. Lists 8 representative plugins that DO parse dates as patterns to follow."
generated: { by: agent/cli, at: "2026-10-28T07:13:59Z" }
status: stable
---

# Related Concepts
- [atsumaru released_at bug: epoch-millis field was never read](atsumaru-released-at-epoch-millis.md): Audit cites this concept: atsumaru-released-at-epoch-millis
- [Recurring plugin failure patterns in goIsekai](plugin-failure-patterns.md): Audit cites this concept: plugin-failure-patterns
- [Odin WASM plugin port candidates (Lua -> Odin)](odin-wasm-plugin-port-candidates.md): Audit cites this concept: odin-wasm-plugin-port-candidates
- [Plugin detail view on /view/plugins (design)](plugin-detail-page-on-plugins-view.md): Live-plugins audit enumerates this plugin/source as part of coverage.
- [Reader two-page view (side-by-side and stacked)](reader-two-page-view.md): Live-plugins audit enumerates this plugin/source as part of coverage.
- [Hostnet CDP timeouts and page waits for SPA sites](hostnet-cdp-timeout-waits.md): Live-plugins audit enumerates this plugin/source as part of coverage.
- [weebcentral.com plugin build spec from weebcentral_com_complete.har](weebcentral-plugin-build-spec.md): Live-plugins audit enumerates this plugin/source as part of coverage.
- [Library duplicate-group colour bands](library-duplicate-colour-bands.md): Live-plugins audit enumerates this plugin/source as part of coverage.
- [1manga: pages field arrives as plaintext JSON, not enc:v1](1manga-plaintext-pages-field.md): Live-plugins audit enumerates this plugin/source as part of coverage.
- [Reader read-ahead verification](doc_reader-read-ahead-verification.md): Live-plugins audit enumerates this plugin/source as part of coverage.
- [Comizy.io plugin build spec](comizy-plugin-build-spec.md): Live-plugins audit enumerates this plugin/source as part of coverage.
- [MangaBuddy returned zero pages for every chapter - stale CDN host + cached empty answer](mangabuddy-empty-chapter-pages.md): Live-plugins audit enumerates this plugin/source as part of coverage.
- [Plugin runtime export matrix (lua vs js vs yaegi vs go vs wasm)](plugin-runtime-export-matrix.md): Live-plugins audit enumerates this plugin/source as part of coverage.
- [Zeroscann search/detail selector findings](zeroscann-search-detail-findings.md): Live-plugins audit enumerates this plugin/source as part of coverage.
- [Host helper surface registry](host-helper-surface-registry.md): Live-plugins audit enumerates this plugin/source as part of coverage.
- [Chapter lists came out oldest-first for weebcentral and comizy](chapter-order-newest-first-abi.md): Live-plugins audit enumerates this plugin/source as part of coverage.
- [Detail-page read toggle, genre tags, and reader-prefs persistence](detail-page-read-genre-readerprefs-fixes.md): Live-plugins audit enumerates this plugin/source as part of coverage.
- [Plan — Host-native helpers for JS/Lua plugins](doc_host-native-helpers-plan.md): Live-plugins audit enumerates this plugin/source as part of coverage.
- [UI Polish Review](doc_ui-polish.md): Live-plugins audit enumerates this plugin/source as part of coverage.
- [Enhance status startup log](enhance-status-log.md): Live-plugins audit enumerates this plugin/source as part of coverage.
- [ABI extension: Manga.Rating + Chapter.Scanlator](abi-extend-rating-scanlator.md): Live-plugins audit enumerates this plugin/source as part of coverage.
- [Comix.to plugin build spec](comix-plugin-build-spec.md): Live-plugins audit enumerates this plugin/source as part of coverage.
- [Cover refresh vs clear actions](cover-refresh-vs-clear-actions.md): Live-plugins audit enumerates this plugin/source as part of coverage.
- [A plugin reads its own volatile facts from plugin.json](plugin-json-params.md): Live-plugins audit enumerates this plugin/source as part of coverage.
- [Sync upserts chapters but never pruned superseded rows - 196 removed](stale-chapter-rows-prune.md): Live-plugins audit enumerates this plugin/source as part of coverage.
- [goIsekai agent handover](doc_handover.md): Live-plugins audit enumerates this plugin/source as part of coverage.
- [Enrichment (info) scripts run as Odin wasm — discovery load-path fix](info-scripts-as-odin-wasm.md): Live-plugins audit enumerates this plugin/source as part of coverage.
- [Onisaga Cloudflare challenge behavior](onisaga-cloudflare-challenge.md): Live-plugins audit enumerates this plugin/source as part of coverage.
- [lhtranslation decodeString - sandbox passes a bare string, not JSON-encoded](lhtranslation-decodeString-sandbox-string-format.md): Live-plugins audit enumerates this plugin/source as part of coverage.
- [Wails desktop client + tray wiring](wails-desktop-client-and-tray.md): Live-plugins audit enumerates this plugin/source as part of coverage.
- [WASM backend sizes and the TinyGo port (Go vs TinyGo vs C++ vs Odin)](wasm-backend-sizes-and-tinygo-port.md): Live-plugins audit enumerates this plugin/source as part of coverage.
- [Wasm plugin local helpers decision](wasm-plugin-local-helpers-decision.md): Live-plugins audit enumerates this plugin/source as part of coverage.
- [WASM host helpers](doc_plugin-wasm-helpers.md): Live-plugins audit enumerates this plugin/source as part of coverage.
- [Test coverage gaps in internal/pluginmanager](pluginmanager-test-coverage-gaps.md): Live-plugins audit enumerates this plugin/source as part of coverage.
- [Session dailies 2026-08-29 to 2026-09-30](session-dailies-2026-08-29-to-09-30.md): Live-plugins audit enumerates this plugin/source as part of coverage.
- [Dead code and merge candidates audit (2026-10-17)](dead-code-and-merge-audit-2026-10-17.md): Live-plugins audit enumerates this plugin/source as part of coverage.
- [go-dl Go launcher reference](go-dl-toolchain-reference.md): Live-plugins audit enumerates this plugin/source as part of coverage.
- [The released_at column was added on 20 Oct but the rows synced before it were never backfilled - the plugin was always right](released-at-backfill-gap.md): Live-plugins audit enumerates this plugin/source as part of coverage.
- [Installing a wasm plugin deadlocked the host - ensureWasmRuntime took the manager lock its caller already held](wasm-install-deadlock.md): Live-plugins audit enumerates this plugin/source as part of coverage.
- [goIsekai JSON API](doc_api.md): Live-plugins audit enumerates this plugin/source as part of coverage.
- [Home reload NS_BINDING_ABORTED (saved-filter redirect)](home-reload-ns-binding-aborted.md): Live-plugins audit enumerates this plugin/source as part of coverage.
- [MangaTaro.org API endpoints](mangataro-api-endpoints.md): Live-plugins audit enumerates this plugin/source as part of coverage.
- [Native .so plugins are self-contained with zero DT_NEEDED entries](static-musl-so.md): Live-plugins audit enumerates this plugin/source as part of coverage.
- [VioletManga search/detail selector findings](violetmanga-search-detail-findings.md): Live-plugins audit enumerates this plugin/source as part of coverage.
- [Bulk "set this and before/after as read" hit the whole manga when chapter_num is 0](chapter-bound-zero-chapter-num.md): Live-plugins audit enumerates this plugin/source as part of coverage.
- [goIsekai UI Specification](doc_ui-spec.md): Live-plugins audit enumerates this plugin/source as part of coverage.
- [library_fts index sync and auto-heal](library-fts-index-sync.md): Live-plugins audit enumerates this plugin/source as part of coverage.
- [1manga runs as a native .so plugin](native-1manga-plugin.md): Live-plugins audit enumerates this plugin/source as part of coverage.
- [1manga search returned 0 results - two stacked GraphQL syntax bugs](1manga-two-stacked-graphql-bugs.md): Live-plugins audit enumerates this plugin/source as part of coverage.
- [Multi-Worker Architecture](doc_feat_multiworker_arch.md): Live-plugins audit enumerates this plugin/source as part of coverage.
- [Plugin Playground](doc_plugin-playground.md): Live-plugins audit enumerates this plugin/source as part of coverage.
- [Library view URL is the source of truth (no localStorage filter persistence)](library-view-url-is-source-of-truth.md): Live-plugins audit enumerates this plugin/source as part of coverage.
- [Reader loading cancel fix](reader-loading-cancel-fix.md): Live-plugins audit enumerates this plugin/source as part of coverage.
- [Three Lua Plugins Converted to TinyGo WASM](three-lua-plugins-converted-to-tinygo-wasm.md): Live-plugins audit enumerates this plugin/source as part of coverage.
- [1manga: port Lua to Go wasm via TinyGo](1manga-wasm-port.md): Live-plugins audit enumerates this plugin/source as part of coverage.
- [Native .so plugin ABI (purego loader)](native-so-plugin-abi.md): Live-plugins audit enumerates this plugin/source as part of coverage.
- [Native .so plugins build with zig c++ against static musl](native-zig-build.md): Live-plugins audit enumerates this plugin/source as part of coverage.
- [Plugin logo sources and cache](plugin-logo-sources-and-cache.md): Live-plugins audit enumerates this plugin/source as part of coverage.
- [Weebcentral search covers landed on the wrong title - one card renders two img](weebcentral-search-card-cover-alignment.md): Live-plugins audit enumerates this plugin/source as part of coverage.
- [MangaDex API /manga?title= returns 0 for multi-word phrases that the website /search matches](mangadex-api-title-phrase-vs-website-search.md): Live-plugins audit enumerates this plugin/source as part of coverage.

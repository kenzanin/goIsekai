## 2026-10-27
* **Creation**: Documented concept `go-dl-toolchain-reference.md` (go-dl Go launcher reference).

## 2026-10-26
* **Creation**: Documented concept `doc_plugin-wasm-helpers.md` (WASM host helpers).
* **Creation**: Documented concept `doc_ui-spec.md` (goIsekai UI Specification).
* **Creation**: Documented concept `doc_feat_multiworker_arch.md` (Multi-Worker Architecture).
* **Creation**: Documented concept `doc_reader-read-ahead-verification.md` (Reader read-ahead verification).
* **Creation**: Documented concept `doc_host-native-helpers-plan.md` (Plan — Host-native helpers for JS/Lua plugins).
* **Creation**: Documented concept `doc_ui-polish.md` (UI Polish Review).
* **Creation**: Documented concept `doc_plugin-playground.md` (Plugin Playground).
* **Creation**: Documented concept `doc_handover.md` (goIsekai agent handover).
* **Creation**: Documented concept `doc_api.md` (goIsekai JSON API).
* **Creation**: Documented concept `wails-desktop-client-and-tray.md` (Wails desktop client + tray wiring).
* **Creation**: Documented concept `session-dailies-2026-08-29-to-09-30.md` (Session dailies 2026-08-29 to 2026-09-30).
* **Creation**: Documented concept `wasm-backend-sizes-and-tinygo-port.md` (WASM backend sizes and the TinyGo port (Go vs TinyGo vs C++ vs Odin)).
* **Creation**: Documented concept `plugin-runtime-export-matrix.md` (Plugin runtime export matrix (lua vs js vs yaegi vs go vs wasm)).
* **Creation**: Documented concept `plugin-json-params.md` (A plugin reads its own volatile facts from plugin.json).
* **Creation**: Documented concept `weebcentral-plugin-build-spec.md` (weebcentral.com plugin build spec from weebcentral_com_complete.har).
* **Creation**: Documented concept `released-at-backfill-gap.md` (The released_at column was added on 20 Oct but the rows synced before it were never backfilled - the plugin was always right).
* **Creation**: Documented concept `plugin-detail-page-on-plugins-view.md` (Plugin detail view on /view/plugins (design)).
* **Creation**: Documented concept `wasm-install-deadlock.md` (Installing a wasm plugin deadlocked the host - ensureWasmRuntime took the manager lock its caller already held).
* **Creation**: Documented concept `weebcentral-search-card-cover-alignment.md` (Weebcentral search covers landed on the wrong title - one card renders two img).
* **Creation**: Documented concept `atsumaru-released-at-epoch-millis.md` (atsumaru released_at bug: epoch-millis field was never read).
* **Creation**: Documented concept `three-lua-plugins-converted-to-tinygo-wasm.md` (Three Lua Plugins Converted to TinyGo WASM).
* **Creation**: Documented concept `1manga-plaintext-pages-field.md` (1manga: pages field arrives as plaintext JSON, not enc:v1).
* **Creation**: Documented concept `pluginmanager-test-coverage-gaps.md` (Test coverage gaps in internal/pluginmanager).
* **Creation**: Documented concept `chapter-order-newest-first-abi.md` (Chapter lists came out oldest-first for weebcentral and comizy).
* **Creation**: Documented concept `reader-two-page-view.md` (Reader two-page view (side-by-side and stacked)).
* **Creation**: Documented concept `static-musl-so.md` (Native .so plugins are self-contained with zero DT_NEEDED entries).
* **Creation**: Documented concept `native-zig-build.md` (Native .so plugins build with zig c++ against static musl).
* **Creation**: Documented concept `native-so-plugin-abi.md` (Native .so plugin ABI (purego loader)).
* **Creation**: Documented concept `native-1manga-plugin.md` (1manga runs as a native .so plugin).
* **Creation**: Documented concept `library-duplicate-colour-bands.md` (Library duplicate-group colour bands).
* **Creation**: Documented concept `comizy-plugin-build-spec.md` (Comizy.io plugin build spec).
* **Creation**: Documented concept `info-scripts-as-odin-wasm.md` (Enrichment (info) scripts run as Odin wasm — discovery load-path fix).
* **Creation**: Documented concept `hostnet-cdp-timeout-waits.md` (Hostnet CDP timeouts and page waits for SPA sites).
* **Creation**: Documented concept `host-helper-surface-registry.md` (Host helper surface registry).
* **Creation**: Documented concept `detail-page-read-genre-readerprefs-fixes.md` (Detail-page read toggle, genre tags, and reader-prefs persistence).
* **Creation**: Documented concept `dead-code-and-merge-audit-2026-10-17.md` (Dead code and merge candidates audit (2026-10-17)).
* **Creation**: Documented concept `stale-chapter-rows-prune.md` (Sync upserts chapters but never pruned superseded rows - 196 removed).
* **Creation**: Documented concept `onisaga-cloudflare-challenge.md` (Onisaga Cloudflare challenge behavior).
* **Creation**: Documented concept `mangabuddy-empty-chapter-pages.md` (MangaBuddy returned zero pages for every chapter - stale CDN host + cached empty answer).
* **Creation**: Documented concept `home-reload-ns-binding-aborted.md` (Home reload NS_BINDING_ABORTED (saved-filter redirect)).
* **Creation**: Documented concept `chapter-bound-zero-chapter-num.md` (Bulk "set this and before/after as read" hit the whole manga when chapter_num is 0).
* **Creation**: Documented concept `atsumaru-released-at-epoch-millis.md` (atsumaru released_at bug: epoch-millis field was read with the wrong integer width).
* **Creation**: Documented concept `wasm-plugin-local-helpers-decision.md` (Wasm plugin local helpers decision).
* **Creation**: Documented concept `library-fts-index-sync.md` (library_fts index sync and auto-heal).
* **Creation**: Documented concept `plugin-logo-sources-and-cache.md` (Plugin logo sources and cache).
* **Creation**: Documented concept `abi-extend-rating-scanlator.md` (ABI extension: Manga.Rating + Chapter.Scanlator).
* **Creation**: Documented concept `cover-refresh-vs-clear-actions.md` (Cover refresh vs clear actions).
* **Creation**: Documented concept `comix-plugin-build-spec.md` (Comix.to plugin build spec).
* **Creation**: Documented concept `reader-loading-cancel-fix.md` (Reader loading cancel fix).
* **Creation**: Documented concept `enhance-status-log.md` (Enhance status startup log).
* **Creation**: Documented concept `mangataro-api-endpoints.md` (MangaTaro.org API endpoints).
* **Creation**: Documented concept `zeroscann-search-detail-findings.md` (Zeroscann search/detail selector findings).
* **Creation**: Documented concept `violetmanga-search-detail-findings.md` (VioletManga search/detail selector findings).
* **Creation**: Documented concept `1manga-wasm-port.md` (1manga: port Lua to Go wasm via TinyGo).
* **Creation**: Documented concept `library-view-url-is-source-of-truth.md` (Library view URL is the source of truth (no localStorage filter persistence)).
* **Creation**: Documented concept `1manga-two-stacked-graphql-bugs.md` (1manga search returned 0 results - two stacked GraphQL syntax bugs).
* **Creation**: Initialized OKF v0.2 project memory.

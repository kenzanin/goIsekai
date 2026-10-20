# Bundle Update Log

## 2026-10-20

- **Ported `violetmanga` Lua → Odin wasm** (`examples/plugins/wasm/violetmanga_odin/`,
  301 KB). Smallest remaining candidate (174 LOC). The Lua header claims the
  reader needs `host.browser.evaluate`, but that plugin **never calls it** — the
  page list is already in the served HTML inside `ts_reader.run({...})`, so all
  five endpoints are plain HTTP. A/B with `plugin_cache` cleared: **search
  identical (2386 B), detail identical (1077 B), pages identical (1125 B)**,
  chapters 13/13 with identical id+title+url; only `chapter_num` changes (0→13..1),
  the same deliberate addition as earlier ports (Lua leaves it unset, so every
  chapter sorted as 0). Swapped in `app_data/plugins/violetmanga/` under the same
  id and name; the Lua original stays in `examples/plugins/lua/violetmanga/`.
  **Two traps.** (1) The first build dropped `author` because the Lua comment
  says the site has no mangaka — but A/B showed Lua emitting
  `"author":"Serab"` from `.imptdt span.author i`. Lesson: trust the payload, not
  the source comment. (2) Chapter ids contain `/` (`/slug-chapter-N/`) because
  the reader route URL-encodes them, so the sandbox `pages/{chapterID}` route
  (one chi segment) returns `404 page not found` unless the id is URL-encoded.
  That is a sandbox-route limit, not a plugin bug.
  `CGO_ENABLED=0 go test ./internal/pluginmanager` green
  (`TestJSInvokeDeadlineCancelsBlockingFetch` flakes under full-suite load —
  passes 3/3 alone, suite green with it skipped).
- **Re-surveyed what is left to migrate, and corrected three claims that had gone
  stale.** Findings only — no plugin was ported or swapped in this pass.
  (1) **"Semua `gmatch` itu `host.regex.gmatch`" is wrong.** There are 4 native
  `gmatch` sites (`1manga:103`, `mangahub:97`, `violetmanga:164`,
  `weebcentral:116`), and `weebcentral` — already ported — proves it is a
  non-issue: Lua `body:gmatch('id="tag-([^"]+)"')` became Odin
  `find_all(body, 'id="tag-([^"]+)"')`, same pattern, one line. What actually
  matters is the **output shape per capture count** (`pluginutil.RegexFindAll`
  + `wasmRegexRow`): 0 captures → one-element row (whole match), 1 → flat
  `[]string`, N → `[][]string`. With `abi.find_all` / `abi.find_all_pairs`
  centralized, that rewrite now costs **zero helper lines**.
  (2) **Tier B was mislabelled.** `violetmanga` and `mangataro` never call
  `host.browser.*` — the word `browser` only appears in comments. Both are pure
  `host.html.*` + `http.get_body` + `json.*`. So `violetmanga` (174 LOC) is the
  **smallest remaining candidate** and had never been in the Tier-A table at all.
  `comix` is now the only true `browser.*` user, so that path still has no live
  Odin proof.
  (3) **`kaliscan` is dead** (`kaliscan.me` and `kaliscan.io` both return `000`),
  so it joins `zeroscann`/`valirscans` in the rejected table.
  Corrected shortlist: **`violetmanga` → `dynastyscan` → `mangataro` →
  `lhtranslation` → `mangakatana`**. `mangataro` needs a wall clock and
  `os.time()` is not in the wasm surface; probe confirms
  `time.time_to_unix(time.now())` builds for `wasi_wasm32`.
- **Found a bigger opportunity than any Lua port: 7 TinyGo plugins whose C++
  twins are already built but not installed.** `komix_to`, `mangak`,
  `ksgroupscans`, `kunmanga_online`, `mangakakalot`, `manhawato`, `reimanga_net`
  are still Go in `app_data/` (14 500 052 B total) while their `_cpp` builds sit
  ready in `examples/` (2 012 491 B total) — **7.2× smaller, no new code**.
  `reimanga_net` alone is 17.2× (1 834 751 → 106 986).
  **Correction to `wasm-cpp-plugin-toolchain.md`:** it claimed `manhawato_cpp`
  "replaces the Go wasm in `app_data/plugins/manhawato/`". `md5sum` disproves it
  — the installed file (`556a3890…`, 2 164 283 B) is identical to the Go build,
  not the C++ one (`aca7086b…`, 306 538 B). No C++ twin has ever been installed,
  and none has live A/B proof. Swapping one requires the usual port procedure
  (clear `plugin_cache`, then compare search/detail/chapters/pages).
  Size picture that decides Odin-vs-C++ for the rest: Odin ~416 KB, C++ ~313 KB,
  TinyGo ~2.1 MB — so for plugins with **no** twin, Odin stays preferable (no
  wasi-sdk, one build command, shared `abi/`).

## 2026-10-19

- **Normalized the duplicated Odin host-helper wrappers into `abi/`** (8 plugins,
  `examples/plugins/wasm/*_odin/`). Each `main.odin` used to re-declare the same
  ~10 wrappers, so every new port rewrote the `[]string` vs `[][]string` JSON
  branching. 13 procs (`url_encode` — 8 identical copies, 31 call sites;
  `find_all`, `find_all_pairs`, `find_one`, `find_text`, `find_attr`,
  `find_list_text`, `find_list_attr`, `sub`, `date_to_iso`, `host_num`,
  `chapter_num`, `sanitize_title`) now live once in `abi/wasm_abi.odin`, and each
  plugin keeps a **one-line delegator** so no call site changed.
  `decode_id` (4 shapes), `normalize_status` (4) and `parse_time` (2) are
  deliberately left local — they genuinely differ per site.
  **`using import` is not allowed by Odin** (`'using import' is not allowed,
  please use the import name explicitly`), so an unqualified name cannot be
  imported into a plugin's scope — but a **proc alias** works, which is what
  keeps the local names and the zero-churn property.
  `find_all` / `find_all_pairs` stay separate because the host answers a
  1-capture pattern as a flat array and a multi-capture pattern as an
  array-of-arrays; the wrong shape is a **hard** `Unsupported_Type_Error`, not a
  silent zero.
  Kept one `abi/` copy per plugin directory (8 identical copies, one md5) rather
  than a shared `wasm/abi/`: a shared import would force every `build.sh` and
  import line to change for no gain given `using import` is out.
  **Behaviour proof:** the right A/B for a pure refactor is wasm-before vs
  wasm-after, not wasm-vs-Lua (the ports already carry intentional gains —
  `chapter_num` 0→N, `manga_id` `''`→id). With `plugin_cache` cleared and the
  host restarted, `weebcentral` returns **search IDENTICAL (5124 B), detail
  IDENTICAL (782 B), chapters IDENTICAL (101974 B)** →
  `normalization is behaviour-preserving`. All 8 rebuild and pass `build.sh`
  export validation; `CGO_ENABLED=0 go test ./internal/pluginmanager` green.
  One deliberate delta: canonical `host_num` adds `|| raw == "null"` —
  behaviourally identical (`null` fails the `f64` unmarshal either way → 0).

- **Ported `mangafire` Lua → Odin wasm** (`examples/plugins/wasm/mangafire_odin/`,
  428 KB). First Odin port to use `crypto.vrf_sign` (a runtime row taking JSON),
  so it also proves that path works from wasm. Chosen over `mangatown` because it
  is the only remaining candidate with **0 `gmatch`** — every other candidate
  needs 4-8 `gmatch` call sites rewritten to `regex.find_all`, and `gmatch` is
  the documented `wasmHelperGaps` entry. Swapped in `app_data/plugins/mangafire/`
  under the same id and name; the Lua original (`main.lua` + `helpers.lua`)
  stays in `examples/plugins/lua/mangafire/` untouched.
  A/B with `plugin_cache` cleared: `detail` identical on every field, `chapters`
  79/79 with identical id set and identical `chapter_num` sequence (79/79
  `released_at` filled), `pages` 142 URLs carrying the required `Referer`.
  **Three traps, all of which cost iterations.**
  (1) **`strings.to_string(sb)` does not copy** — returning it while
  `defer strings.builder_destroy(&sb)` is armed hands the caller freed memory.
  The symptom is silent: the URL prints correctly, the fetch fails inside wasm,
  and the endpoint answers `[]` with nothing in the host log. The builder is now
  owned by the caller.
  (2) **JSON struct fields must be exported.** Lowercase `items`/`meta`/`hid`
  make unmarshal skip those fields with no error at all, so `len(resp.items)==0`
  and search returns `[]`. Same symptom as (1), different bug — instrument before
  guessing.
  (3) **`slice.sort_by` is not stable**; `slice.stable_sort_by` preserves source
  order for ties (`chapter_num` 37.1 vs 37.1 previously shuffled positions).
  A/B also exposed **two bugs in the Lua original**, which the port deliberately
  does not copy: Lua reads `meta.last_page`/`meta.has_next` and `c.created_at`
  (snake_case) while the API sends `lastPage`/`hasNext`/`createdAt` (camelCase),
  so Lua stopped after page 1 (50 results vs 479) and emitted
  `0001-01-01T00:00:00Z` for every chapter date. Lesson recorded: A/B is not
  "match Lua" — a correct port looks *different* when Lua's bug was masked by a
  default, so check the raw API payload first.

- **Ported `weebcentral` Lua → Odin wasm** (`examples/plugins/wasm/weebcentral_odin/`,
  417 KB). First Odin port of an HTML+HTMX source: it uses the `html.*` group
  plus `regex.find/find_all/replace`. Swapped in `app_data/plugins/weebcentral/`
  under the **same plugin id and `plugin.json` name**, and the Lua original stays
  in `examples/plugins/lua/weebcentral/` untouched (examples/ is an archive —
  only `app_data/` swaps runtimes). A/B against the Lua baseline: `search` 32/32
  identical (id+title+cover), `detail` identical on every field, `chapters`
  574/574 identical on id+title+`released_at`. One deliberate improvement:
  `chapter_num` now 574/574 filled where Lua left every row 0; chapter IDs are
  unchanged so reading progress survives.
  **Two traps worth remembering.** (1) **Lua regex is not portable to RE2 as-is:
  Lua's `.` matches newline, RE2's does not.** The chapter row text wraps across
  lines, so `\s*Last Read.*$` worked in Lua and matched *nothing* in RE2, leaking
  the whole raw multi-line row into the title — fixed with `(?s)`. Caught only by
  comparing all 574 titles, not a sample. (2) **`plugin_cache` makes a port look
  broken.** After the swap `/chapters` still served the cached **Lua** payload (5
  `GetChapterList` rows), so `chapter_num` read 0/574 and I nearly filed it as an
  Odin bug — a host probe proved `text.chapter_num` returns 1/12.5/3 correctly.
  Always `DELETE FROM plugin_cache WHERE plugin_id='<id>';` **before** A/B.
  Also recorded in concept/odin-wasm-plugin-port-candidates.md why `zeroscann`
  (site 500, needs PHP 8) and `valirscans` (returns null) were rejected: check
  the site before porting, helper ranking alone picks dead sources.

- Added concept/host-helper-surface-registry.md: every runtime used to hardcode
  its own helper list (lua_natives.go, js_natives.go, wasm_host_call.go's six
  per-kind maps, yaegi_host.go) and nothing compared them, so JS had silently
  fallen behind Lua — `host.text.trim`, `host.text.unescape` and
  `host.text.lua_escape` were reachable from Lua and wasm but **not** from JS,
  and the only parity test covered Lua↔wasm. Now
  `internal/pluginmanager/host_surface.go` is the single definition: one row per
  helper, `"<namespace>.<helper>"` plus a `helperKind`. `kindPure` rows carry a
  Go function and every runtime wires them automatically (each supplies only its
  own wrapper — `luaStr1`, `jsStr1`, or the wasm dispatcher's direct call);
  `kindRuntime` rows (doc handles, goja objects, `http.*` needing manager+id+ctx,
  `regex.gmatch`) stay per-runtime but are still declared, each with a `note`
  saying why. wasm lost all six `wasmHostStr*` maps down to one
  `hostSurfaceIndex[fn]` lookup (−59 lines), and Lua/JS became table walks.
  yaegi deliberately not migrated: a Go-dialect plugin calls real Go names
  (`hostnet.Get`), not `"text.url_encode"`. New tests: `TestJSHostHelperParity`
  walks the **real** Lua host table and asserts JS reaches every name,
  `TestHostSurfaceRowsAreComplete` requires a function on `kindPure` and a
  reason on `kindRuntime`. Traps: `regex.quote` is pure in both runtimes (so it
  belongs on the table) while `regex.replace` is not, because Lua and JS each
  accept a per-match *function* replacement; Lua/JS `log.*` is not `host.log`
  (Lua builds a `log` table, JS registers `console.*` in `js.go:66` via
  `newJSConsole`), so `jsHelperRenames` records that spelling difference with
  its reason; and a JS parity test must also install `console`/`http_request`,
  which `newJS` does and `registerJSHostNatives` does not. Verified
  `CGO_ENABLED=0 go test ./... -count=1` — 17 packages ok, gofmt clean.
- Added concept/odin-wasm-plugin-port-candidates.md: tier-ranked list of Lua
  plugins that are already ported to Odin in examples/plugins/wasm/*_odin/
  plus the next candidates. 6 buat saat ini terverifikasi A/B identical dengan
  Lua (atsumaru, comicasura, comizy, mangade, mangadex, manga_uno). Tier A
  (tanpa regex): mangatown/108 baris, mangahere/159, fanfox/165, mangafreak/161,
  kaliscan/209, zeroscann/239, weebcentral/283, valirscans/225, mangafire/308.
  Tier B (pakai browser.*): violetmanga/mangataro/comix. Tier C (perlu rewrite
  regex.gmatch → regex.find_all): dynastyscan, en-thunderscan, harimanga, lhtranslation,
  madaradex, mangaread, mangasushi, topmanhua, anisascans, mangabuddy, mangakatana,
  mangahub, 1manga. Performance truth: di JSON API endpoint (mangadex A/B),
  wasm ~0.18s vs Lua ~0.075s — wasm tidak lebih cepat di sana, perbedaan terasa
  di HTML berat + regex di mana Odin kompilasi ungguli interpreter Lua.

## 2026-10-11

- Bundle initialized with `pi-okf init`.
- Added concept/cover-refresh-vs-clear-actions.md: Refetch re-downloads the main cover (busts caches), Clear drops the alt_covers candidate list (toast `Cover candidates cleared`, evidence cover_clear_refresh.har).
- Converted EchoesVault knowledge pages to concepts:
  - concept/enhance-status-log.md: startup image-enhance status log (formatEnhanceStatus/LogEnhanceStatus wiring, log format examples, edge cases).
  - concept/reader-loading-cancel-fix.md: reader loading cancel via AbortController (reader.js abortLoad, bars visible during load, reader.lua spinner display:none).
  - concept/library-fts-index-sync.md: FTS5 CAST key-comparison fix, 7 self-syncing mutation choke points, LibraryFTSDrift/EnsureLibraryFTS auto-heal, 10k-row INTEGER-key ceiling.
  - concept/wasm-plugin-local-helpers-decision.md: DECISION keep local helpers on wasm hot paths — hostCall IPC+JSON roundtrip cost, status map is plugin data, coregex/go-json size tradeoff.
- Consolidated 8 EchoesVault daily scratchpads into concept/session-dailies-2026-08-29-to-09-30.md (tls-client migration, human-verify/WebP ship, image burst pacing root cause, tailwind path bug, wasm host helpers + plugin ports, decimal-chapter audit, FanFox findings).
- Added concept/detail-page-read-genre-readerprefs-fixes.md: three bugs that all looked frontend-only. (1) the chapter row renders the **derived** `Done` flag (`IsRead || (TotalPages>0 && LastPageRead>=TotalPages)`) while `ToggleChapterRead` flipped only `is_read`, so a fully-read chapter could never be un-read — now the toggle clears `last_page_read` too, but only when the row was fully read, so a partial bookmark survives. (2) `cachedMangaFallback` — the path the detail page always takes for a library manga with persisted chapters — set neither `RawGenres` nor `Genres` (without an override), so no genre tags rendered and the enrichment panel marked no chip active, making every chip's click toggle off-then-on; the enrichment categories are the plugin genres' persisted mirror, so the cached path now uses them. (3) `reader.js` debounced prefs 600ms with no unload flush (a chapter switch reloads the page, dropping the write) and `restoreZoom` read the stale page-load server snapshot first (snapping zoom back each page turn) — now `flushPrefs()` POSTs with `keepalive` on `pagehide`/`visibilitychange` and the local mirror wins. Each fix ships a regression test (`TestToggleChapterReadResetsFullyReadChapter`, `TestCachedMangaFallbackPopulatesGenres`, `TestReaderPrefsSurviveAChapterSwitch`). Verified live: toggle round-trips, detail page renders 9 genre chips, and prefs changed inside the 600ms debounce still persist across an immediate chapter switch.

## 2026-10-18

- Added concept/home-reload-ns-binding-aborted.md: **finding only, no fix applied.** A library reload logs `NS_BINDING_ABORTED` in Firefox against plugin logos/favicon, but `NS_BINDING_ABORTED` is not an HTTP status — it is Firefox's internal "this navigation was cancelled", recorded in a HAR as `"status": 0` with `time: 0`, `timings: {}`, `bodySize: -1`. So a manual 200 is expected; the server never failed. Cause: `alpine-components.js` runs `syncLibraryView()` synchronously at script parse (no `defer`), and on a bare `/` with a saved `localStorage['gsk:library-view']` it calls `window.location.replace('/?sort=recent_read')`, cancelling the in-flight first navigation and every subresource it had started. In `start.har` exactly one entry is aborted (favicon, referer `/`); all 28 `plugin-static/*/logo.png` are 304 and the favicon is re-fetched at 200 with referer `...?sort=recent_read`. Impact: none — the aborted requests belong to the page being replaced, and the final page loads its own favicon/logos. Reproduced live (set the localStorage key, load bare `/` → tab jumps). Two fix options recorded and deliberately **not** applied (user judged it harmless): (1) persist the filter in a cookie + `http.Redirect` 302 in the root handler — one navigation, no abort, cost a cookie + handler change; (2) defer the JS redirect until load finishes — no architecture change, but flashes the unfiltered grid. Do not "fix" it by making assets synchronous or dropping `no-cache`; neither is related.
- **Replaced the `.so` loader: stdlib `plugin` → `github.com/ebitengine/purego`.** stdlib `plugin` requires cgo plus an identical toolchain while the host builds `CGO_ENABLED=0`, so the `go` kind had never loaded a single library (`loadGo`/`callGo` sat at 0% coverage). purego reaches dlopen through the platform loader (`dlfcn_nocgo_linux.go`), so the host stays cgo-free; it was already in `go.sum` as an indirect dep and `go mod tidy` promoted it to a direct one. Added concept/native-so-plugin-abi.md. **ABI:** `char *(const char *)` JSON in/out per entry point, **plugin allocates its result and the host calls a required `plugin_free(ptr)` after copying** (purego never frees a returned `char*`, so ownership had to be explicit rather than guessed per language), and failures travel as the shared `{"error":"code"}` envelope so `PluginError()` renders them. Input needs no manual buffer — purego NUL-terminates a Go `string` argument itself. Metadata stays in `plugin.json` and `discoverGo` never opens the library, so `.so` plugins are as lazy as every other runtime. `cString` walks bytes manually because purego keeps `CString`/`GoString` internal, and its `uintptr`→`unsafe.Pointer` conversion needs a reasoned `//nolint:govet`. Fixture `examples/plugins/native/dummy` (C++ `.so`, canned answers, echoes the query back, one entry point returns the error envelope); `*.so` is gitignored so the test **skips** without it. **Trap found while testing:** return shapes must match the host decoders — `Search`/`GetChapterList` decode a *bare array*, `GetMangaDetail` a *bare object*, and `Chapter.ChapterNum` is a *number*; the fixture first returned the HTTP-API shape `{"results":[...]}` and `"chapter_num":"1"`, which the tests caught.
- Added concept/pluginmanager-test-coverage-gaps.md: after the coverage pass in `e36c6f2` (60.0% → 70.7%) this records what is still unreached and why. `loadGo`/`callGo` are 0% because the `.so` kind has never loaded (stdlib `plugin` needs cgo, host builds `CGO_ENABLED=0`) — the gap the purego loader targets. The wasm *host* side (`hostHTTPRequest`, `hostCall`, `writeWasmResult`, `wasmHostError`, `wasmHTTP`, `wasmVRFSign`) and the Lua native error paths (`errorTable`) are the other blocks. Also records two known live defects tests do not catch: `comix` search returns 0 because its HTML parser no longer matches the site (fetch itself works — 28 380 bytes — and sibling `komix_to` gets results via the JSON API), and `TestSamePriorityFIFO` is a timing flake (failed once in a full run, passed 5/5 alone).
- **Native plugins can now reach every host helper** (concept/native-so-plugin-abi.md). A wasm guest gets them through the `env.host_call` *import*, but a `.so` has no import table — so the direction is reversed: the host calls an optional `void plugin_init(host_call_fn fn)` export at load time with a `purego.NewCallback` function pointer, and the plugin sends back the **same envelope wasm uses** (`{"fn":"text.url_encode","args":[...]}`). One bridge reaches all of them because `dispatchHostCall` is already the single dispatch point, and **no allocation crosses the two allocators** — the host writes into a buffer the plugin owns (`req, out, cap`, with `-1` meaning "retry bigger"). `plugin_init` is optional, so existing `.so` plugins still load without helpers. Plugin-side half is the new `examples/plugins/native/dummy/native_abi.h` (the `.so` counterpart of `wasm_abi.h`: same helper names `http.get_body`/`http.post_body`/`host_str`, buffer doubling 64 KiB → 8 MiB for full HTML pages). Also fixed a claim in the same doc: `callGo` *does* now bound the helpers (it publishes its ctx on `p.curCtx`, which the bridge reads directly — locking `p.mu` there would deadlock since the callback runs on the same goroutine that holds it); only the C call itself stays uninterruptible. Test `TestNativePluginReachesHostHelpers` is the proof — the fixture asks the host to `text.titlecase` "host bridge" and returns `"Host bridge"` in `description`, so an empty value means the bridge was never wired. `ponytail:` one append-only callback slot per load (purego's table is `maxCB = 2000` with no release API), so only frequent `ReloadPlugin` calls would ever exhaust it.

- **Fixed native/wasm plugin reload** (concept/native-so-plugin-abi.md). `ReloadPlugin` re-implemented re-instantiation by hand and passed `p.wasmPath` to the folder-based `LoadPlugin` — but for `.so` and wasm plugins `wasmPath` is the entry *file*, and `LoadPlugin` probes only `main.lua`/`main.js`/`main.go`/`main.wasm`, so reload could never re-open either kind (`no main.lua, ... or main.wasm found at app_data/plugins/roliascan/main.so`). Worse, it `delete`d the registry entry **before** loading, so a failed reload removed a running plugin until restart — reproduced live against roliascan. Now `ReloadPlugin` is just `UnloadPlugin` + `ensureLoaded`, which also means the native `dlclose` actually happens: `dlopen` of an already-loaded path returns the cached handle and keeps executing the old code, so a reload without release would silently run the version it started with (proven by measurement). Failure now leaves the plugin registered-but-unloaded, recoverable by fixing the file. No file watcher exists and none is wanted: with lazy loading the only reload triggers are `/api/sandbox/plugins/{id}/reload`, install, and hot-load. Decision recorded — **native plugins stay lazy**: first `dlopen` is 7.7 ms but that is the shared `libcurl`/`libxml2`/`libcrypto` cost paid once per process, and subsequent loads are 0.3 µs, so eager loading would buy nothing while dropping a real .so failure into startup.

## 2026-10-17

- Removed dead code tier-1 in commit `ec12590`: ten symbols `deadcode -test ./...` flagged as unreachable even from tests — `AppService.{Log, RecordRead, GetMangaDetailsCached, ClearPluginLogo, EnrichmentCatalog}` plus the now-orphaned `EnrichmentCatalogEntry` type, `AppService.{SetMangaGenres, AddGenre, RemoveGenre}`, `LuaEngine.Has`, and `config.LoadOrInit` (which took its whole file `internal/config/loader.go`). 158 lines deleted, 0 added; `deadcode -test` 12 → 2. Each symbol was checked for dynamic dispatch first (no `reflect.MethodByName`, no Wails `Bind` — Wails was dropped in the archived openspec — and no name used as a plugin/host-call string), which matters here because `deadcode` cannot see yaegi/wasm/template calls. Left for a decision (tier-2): `internal/bridge/migrate_candidates.go`, `workers.Permanent` (a real bug — see the audit concept), `database.NormalizeTitle`, `ImageFormat.FormatExtension`.
- Added concept/detail-page-read-genre-readerprefs-fixes.md: the three reader/detail bugs plus the follow-up where the chapter-list bulk unread actions turned out to have the same root cause as the per-row toggle.
- Added concept/reader-two-page-view.md: the reader's two-page layouts for sources that split one original page into two images. `spread` is a tri-state (`''`/`side`/`stack`), cycled with every other read mode by one A/B/C/D button (`#btn-readmode`; A 1 page · B 2 stacked · C 2 side by side · D continuous strip) — `side` puts the halves left/right (rtl reverses order), `stack` puts them one above the other so they read as one continuous page. Records the contracts that keep it working (`stepSize()` = 2 per step, trailing odd page stands alone, strip mode wins, counter shows a range, `viewBox()` scales the pair as one unit), the measured canvas geometry that tells the two layouts apart (`side` 2083x1600 vs `stack` 1125x2720 for pages 958x1120 + 1125x1600), the `img2` resets, and the opaque-JSON per-manga `spread` pref.
- Added concept/library-duplicate-colour-bands.md: the library grid flags duplicate groups with a 3px left colour band, one colour per group (`dupPalette` in `views_library.go`, indexed by group position → `DupColors`). Records why the band is an inline `style` and not a Tailwind class (Tailwind can only generate classes it can see spelled out; `"border-" .. color` renders nothing), why `border-left` over `ring`/full `border` on a `rounded-lg overflow-hidden` card, the palette rules (readable on `bg-neutral-900`, must avoid the indigo accent), the new `?sort=duplicates` option, and the determinism the colours depend on: `FindPotentialDuplicates` now sorts equal-size groups by `Key` (it previously used a bare `sort.Slice` with no tiebreak, so bands could recolour between restarts). Verified by restarting twice and diffing the band colours.
- Added concept/wasm-cpp-plugin-toolchain.md: goIsekai WASM plugins can be written in C++17 and built with the system wasi-sdk clang (`--target=wasm32-wasi -mexec-model=reactor`, RapidJSON, `-idirafter /usr/include`). Covers the `libxml2.so.2` shim for wasm-ld, the duplicate-export trap (`_initialize`/`malloc`/`free` come from the reactor CRT), the three reusable headers (`wasm_abi.h`, `util.h`, `html_util.h`) shared by `komix_to_cpp` and `manhawato_cpp`, host-helper delegation via `host_str` (`text.url_encode`/`url_decode`/`date_to_iso`/`strip_html`/`unescape`), and the measured size comparison: C++ ~0.3 MB vs TinyGo ~1.9 MB vs standard Go 3.9-5.2 MB. Shipped commit a628bd5; `manhawato_cpp` replaces the Go wasm in `app_data/plugins/manhawato/`.
- Added concept/dead-code-and-merge-audit.md: swept for dead code + merge candidates. `deadcode ./...` = 19 hits, `-test` = 12 (12 symbols dead even to tests, 7 test-only). Biggest block `internal/bridge/migrate_candidates.go` (198 lines, no production caller; `views_migrate.go` re-implements its parallel search inline). Records the real bug found: `workers.Permanent()` is checked in `enqueue.go:238` but never called in production, so the "no retry on permanent 404" path is wired at one end only. Six diff-verified merge candidates (duplicate genre handlers, twin `Enqueue*` helpers, 3-layer `NormalizeTitle`, 31 resolve-id→db→wrap methods, `FormatExtension` alias). Also records the tooling gotcha: `deadcode`/`staticcheck` in `~/go/bin` were go1.26-built and could not parse this go1.27 module — rebuilt via `go install ...@latest`. Findings only, nothing applied.
- Ignore `.codegraph/` in git (commit `b2db5e7`); per-machine regenerable index, same as `.codebase-memory/`.
- Applied all six merge candidates from concept/dead-code-and-merge-audit.md (see the doc for per-item status): `handleToggleGenre` collapses the twin genre handlers; `enqueueFanout[T]` carries both `Enqueue*` fan-out helpers; the migrate view now calls `CollectMigrationCandidates` (alt-title matching live — `deadcode -test ./...` is now 0, `deadcode ./...` leaves only `AutoSelectCandidate`); `database.normalizeTitle` pass-through deleted (5 sites call `pluginutil.NormalizeTitle` directly); `withManga`/`withChapter` in `bridge/service.go` absorb 11 resolve→db→wrap sites with byte-identical error strings (multi-step/value-return/`MangaRowID`/`ToggleLibraryItem` deliberately untouched — a single-stage helper would change their error shapes); `ImageFormat.FormatExtension` deleted with its test repointed. Plus the `workers.Permanent` producer fix: `permanentIfFinal` wraps only `*hostnet.ChallengeError` on `runOnFetch`/`EnqueueExportCBZ`, proven by `TestRunOnFetchDoesNotRetryAChallenge` (1 attempt vs 3).
- Settled every normalization candidate in concept/plugin-runtime-export-matrix.md (see its "Resolution" section). **Metadata is now `plugin.json` for every runtime**: all five loaders stopped parsing meta out of the plugin (lua `PLUGIN` table, js `PLUGIN` object, yaegi `Init()`+JSON, go `Meta` var) and read `plugin.json` via the shared helper — what wasm already did. That is why `LoadPlugins()` used to return an empty `Name`/`Logo`/`SiteURL` for lua/js/yaegi/go until first invoke, and why gated image CDNs lost their Referer. `PluginMeta` gained `Description`/`ContractVersion`; a plugin declaring no version is treated as current, not v0. Migration shipped `plugin.json` for 37 installed + 51 example plugins + info scripts (`TestEveryShippedPluginDeclaresMetadata` guards it) and caught `manhawato`, which had only `site_url` and would have shown nameless. ABI now: Search is `search` in Lua/JS (was `search_manga`/`searchManga`), `get_enrichment` moved into its own `luaInfoFnNames` map so a source plugin is never verified against an enrichment export, and `GetMangaDetailWithChapters` (documented + implemented but mapped by no runtime, 0 callers) is gone. Legacy ABI names still resolve with a deprecation warning, so the 64 Lua + 6 JS plugins needed no migration. `deadcode -test` went 12 → 2. **Ordering trap worth remembering: the `plugin.json` migration MUST precede stripping the in-plugin parsing** — `loadLua` required the `PLUGIN` table and `lifecycle.go` overwrites the discovery-seeded meta with runtime meta, so stripping first silently drops `verify_url`/`thumb_ratio`/`timeout` for every plugin.
- Added concept/plugin-runtime-export-matrix.md: traced "kenapa wasm tidak export function Init?" into a full five-runtime comparison. Answer: the wasm loader never looks for `Init` (the `_initialize` it calls is the WASI reactor CRT entry, not the ABI) — wasm meta comes from `plugin.json`, per the comment in `discovery.go:18`. Matrix covers ABI export names per runtime (`search_manga`/`searchManga`/`Search`/`Search`/`Search`), return convention (lua accepts a table, wasm is always JSON over linear memory), meta source (`PLUGIN` table / `PLUGIN` object / `Init()` / `Meta` var / `plugin.json`), and contract-version enforcement. Records six normalization candidates: `abi.go:25` documents `Init` as universal though only yaegi honours it; `GetMangaDetailWithChapters` is documented and implemented but mapped by **no** runtime (always falls back, 0 production callers); JS `CheckVersion(int32(1))` is a hardcoded literal so it can never fail; `GetEnrichment` is only mapped for lua; the wasm host-helper surface is ~25 `host_call` keys versus lua's ~40 flat globals plus tables; and the three naming conventions.


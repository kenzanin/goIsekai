# Bundle Update Log

## 2026-10-21

- **The whole library was re-synced through the button that already exists: `released_at` NULL went 26,145 -> 7,913.** Follow-up to the 1manga finding below, after asking "ada tombol untuk re-sync di ui bro?" and "jadi seharusnya udah tidak perlu dibuatkan lagi bro ya?". Answer: no new code was needed - the UI already has **Update** on the library page (`POST /action/sync` -> `SyncLibrary()` -> `syncMangas()`) and **⟳ Update** on a manga detail page (`POST /action/sync-manga/{pluginID}/{mangaID}`), plus an hourly scheduler (`StartLibraryScheduler` -> `SyncStaleLibrary`). The library-wide button was used as-is: 519 manga in one sequential pass on the fetch lane, **355 seconds**, 453 re-synced (the other 66 failed on an anti-bot challenge - comix, onisaga - which has nothing to do with the date), taking `released_at` NULL from **26,145 to 7,913**. **The scheduler would not have done it:** it only refreshes manga whose `updated_at` is older than `update_stale_days` (3 by default), and after the 20 Oct migration only **12 of 519** library manga were stale while 507 were fresh - so it would have left almost the whole gap untouched. The button is the right tool. **The remaining 7,913 NULLs are not a backfill gap any more**, they are rows the source has no date for: **1,570** old rows whose plugin changed its chapter-id format (lhtranslation now emits `<slug>:chapter-53` where it used to emit bare `chapter-53`, so a re-sync creates the new row and orphans the old one - `syncMangas` upserts but never prunes), **4,503** rows from plugins that never send the field (bato1 2,341, mangakatana 886, mangafreak 816, ksgroupscans 305, topmanhua 110, comix 40, onisaga 5), **1,837** zero-time rows (mangaread, mangzio, anisascans answer `"released_at": "0001-01-01T00:00:00Z"`, and manga_uno sends 74/74 the same way - the site has no date and the host stores that as NULL on purpose), and 3 misc. **Rule:** an empty `released_at` means "this row has no date the site is willing to give", not "the plugin is broken".

## 2026-10-21

- **An empty `released_at` was old rows, not a broken plugin.** Reported: "bro 1manga untuk status release_at nya kosong itu bug atau bukan?". The 1manga plugin returns the date on every chapter - `GET /api/sandbox/plugins/1manga/chapters/martial-peak` gives **3,865/3,865** with `released_at`, and another manga 109/109 - while SQLite had 2,489 NULL rows. The split in the database is a clean time boundary, not a scatter: the NULL rows were fetched 2026-10-18 20:13 → **2026-10-20 15:58** and the filled ones **2026-10-20 17:01** → 2026-10-21, and the commit that added the dedicated column (`79fc8f0 fix(database): store the site's chapter upload date in its own column`, 2026-10-20 23:44 +0700 = **16:44 UTC**) lands exactly between the two. So the column was added but the rows synced before it were **never backfilled** - the concept `chapter-release-date-column.md` describes the change, this is the gap it left. The decisive test: re-syncing one manga with NULL dates through `POST /action/sync-manga/1manga/<slug>` filled all five of its chapters at once, so the plugin had the dates all along, and `UpsertChapter` carries `released_at` in **both** `DO UPDATE` clauses, so a re-sync refreshes the date in place. **Fixed** by re-syncing the 50 affected 1manga library manga (one at a time - the fetch lane is a single worker, a burst only queues behind itself), which took the count to **0 NULL out of 2,799 chapters**; the detail page now renders the dates (`/view/manga/1manga/shadow-skill`, 92 chapters) and `released_at` is the site's own date while `fetched_at` is the sync time (`2022-04-24 21:56:04` vs `2026-10-21 06:23:58`). **Not only 1manga:** every plugin with chapters synced before the 20 Oct migration has the same gap, and the ones whose build does emit the field are backfillable the same way (verified with `strings` on the installed binary): mangafire 5,376, atsumaru 5,703, mangak 1,984, mangaread 1,442, mangadex 1,342, mangahub 1,099, mangabuddy 1,088, lhtranslation 715 - while ksgroupscans (305) genuinely never sends a date. **Rule:** an empty `released_at` means "this row was synced before the column existed", not "the plugin does not know the date". The UI (`internal/templates/partials/detail_chapters.lua:145`) only renders a date when `c.ReleasedAt` is present and does not start with `0001`, so a NULL row shows no date at all - there is no placeholder to look like a bug. Added concept/released-at-backfill-gap.md.

## 2026-10-21

- **1manga now runs as a native .so plugin.** Request: "buatkan native plugin untuk 1manga dan gantikan prod plugin ingat jangan di hapus yang di example". New `examples/plugins/native/1manga/` (main.cpp 570 lines, util.h, native_abi.h, build.sh, plugin.json, logo.png). The **Lua original in `examples/plugins/lua/1manga/` is untouched** (git status for that path is empty, all three files still there) - the native source lives in a separate directory, so nothing was deleted from the archive. The prod plugin was swapped: `app_data/plugins/1manga/` now holds `main.so` (394,800 B) plus the native `plugin.json`, and the Lua file was **renamed** to `main.lua.lua-bak` rather than removed - Lua discovery runs *before* the native glob, so a live `main.lua` would have kept claiming the id and the `.so` would never have loaded. The DB row now points at the new entry file (`1manga|1Manga|app_data/plugins/1manga/main.so`). The port keeps the Lua behaviour exactly: the MangaHub GraphQL API at api.mghcdn.com (source id mn03, Origin/Referer headers or the API answers 404), the `mhub_access` key minting via `/chapter/<slug>/chapter-<n>?reloadKey=1` with a never-issued cookie value, the retry-on-refusal loop (MangaHub reports an expired key and its "API rate limit excessed" as **HTTP 200 with an `errors` array and a null payload**, so the status code alone cannot tell a good answer from a refusal), the encrypted page list `enc:v1:<keyId>:<iv>:<tag>:<ciphertext>` with the key from `/api/chapter-crypto` (which authenticates on the **cookie**, not on the `x-mhub-access` header the GraphQL call uses) and the decryption through the host's `crypto.aes_gcm_decrypt`, the CDN-probing fallback and the newest-first chapter order. **A/B against the live site, same host, Lua build vs native build:** `search?q=one+piece` **byte-identical** (4,294 B), `detail/martial-peak` **byte-identical** (775 B), `chapters/martial-peak` 3,865 rows identical **including `released_at`** (0/3865 differ), `pages/martial-peak:chapter-59` **byte-identical** (1,343 B, 20 pages, same CDN URLs). The AES path works: the log says "1manga pages: found 20 pages for martial-peak:chapter-59 (decrypted)", and the host loads the plugin as `kind=go`. **The trap that broke it:** the first live `GetPageList` failed with `plugin 1manga: invalid GetPageList result: expected comma after object element` - not the site, not the bridge: **RapidJSON's `AddMember(key, Value(double))` emits `{"index":0.0}`**, and Go's decoder rejects that when the field is an `int` (`types.Page.Index`), so one field kills the whole page list. `util.h` now writes an integral value as a JSON **integer**, and the cast must be `int64_t`, not `long long` (RapidJSON's constructors take `int64_t` which is `long` here; `long long` matches several equally and the build fails with "ambiguous conversion for functional-style cast"). The existing native plugins get away with a double because their numeric fields are `float64` (`chapter_num`) or absent - `index` is the first `int` field a native plugin has to emit. **Build:** `tools/build-native.sh` tier 1 (zig c++ + static musl) -> **404,312 B**, `readelf -d` shows **no DT_NEEDED entry at all**, `nm -D` exports all 7 ABI symbols, cold build ~6 s. **Gate:** `git diff --check` clean, `gofmt -l internal/ cmd/` empty, `go vet ./...` clean, `golangci-lint run ./...` 0 issues, `bash -n` clean. `go test ./...` fails on **`TestSamePriorityFIFO`** ("expected 3 requests, got 2") - but the Go tree is **identical to HEAD** (`git diff HEAD -- '*.go'` is empty) and the test fails 5/5 in isolation with the server stopped, so it is **pre-existing**, not a regression from this port (AGENTS.md lists it as a known flake). Added concept/native-1manga-plugin.md.

## 2026-10-21

- **Native .so plugins are now fully self-contained: zig c++ against static musl.** Request: "jangan pakai gnu libc bro untuk zig nya gunakan musl libc check lagi berapa size nya?" - the follow-up to the zig c++ build. `tools/build-native.sh` now has three tiers, best first: (1) **zig c++ + static musl**, (2) zig c++ (glibc), (3) g++. Tier 1 produces a shared object with **zero `DT_NEEDED` entries** - it carries its own libc, so the host needs no libc at all, neither glibc nor musl. **Size:** roliascan 137,560 B (g++) -> 290,416 B (zig glibc) -> **376,264 B** (musl); madarascans 273,816 -> 572,688 -> **696,424**; dummy 79,920 -> 256,768 -> **328,048**. ~30 % over the glibc build, ~2.7x g++ - the price of a libc inside the file. **The recipe:** `zig c++ -target x86_64-linux-musl -idirafter /usr/include -std=c++17 -O2 -fPIC -ffunction-sections -fdata-sections -c *.cpp` then link with `-Wl,--gc-sections -Wl,-Bstatic /usr/lib/musl/lib/libc.a -Wl,-Bdynamic`. **The HOST's musl, not zig's:** zig's bundled musl `libc.a` is **not PIC** and cannot go into a `.so` (`relocation R_X86_64_32S cannot be used against local symbol`, `__utc`); the distro's musl (`musl-tools`) ships a PIC `libc.a` and works. **Three traps, each found by crashing rather than by reading:** (1) `-I/usr/include` puts glibc's `<string.h>` ahead of libc++'s and `<cstring>`/`<cwchar>`/`<cstdio>` fail with *"tried including <wchar.h> but didn't find libc++'s <wchar.h> header"* - use `-idirafter`; (2) a `--version-script` limiting exports to the 7 ABI names (343,096 B, tempting) **segfaults** on the first call inside `purego.RegisterFunc`; (3) `-Bsymbolic` **segfaults** the same way - both force the `.so` to bind `malloc` to its own musl copy, and musl's `__init_libc` never runs for a `dlopen`'d `.so`. The plain build is safe because `RTLD_LOCAL` keeps the `.so`'s definitions out of the global scope. So: do **not** add `-Bsymbolic` or a version script here. A **dynamically** linked musl `.so` is useless on a glibc host: `NEEDED: libc.so` and the host's `/usr/lib/libc.so` is a linker script, not an ELF (`invalid ELF header`); bundling musl's `libc.so` beside it with `-rpath $ORIGIN` does load but puts two libcs in one process and segfaults. **Proof:** `readelf -d` shows no `DT_NEEDED` on all three plugins; `nm -D` still exports the 7 ABI names (`Search`, `GetMangaDetail`, `GetChapterList`, `GetPageList`, `GetGenres`, `plugin_init`, `plugin_free`); the 5 native ABI tests **PASS**; live on the running host (Go + purego `dlopen`): `plugin loaded (lazy) id=roliascan kind=go version=1`, `search?q=one+piece` 200 (677 B), `detail/10724` 200 (1,089 B), `chapters/10724` 200 (90,743 B, 500 rows) - same byte sizes as the glibc build. `pages/10724` 500 is a **wrong id** (10724 is a manga id; the chapter id is 332027), not a build problem. **Stamp fix:** `musl-gcc --version` prints the *GCC* version, so `built_with` lied; musl's version is now read from `/usr/lib/ld-musl-*.so.1` and the badge reads `zig c++ musl 1.2.6`. Added concept/static-musl-so.md and corrected the "full static is not reachable" claim in concept/native-zig-build.md.

## 2026-10-21

- **Native .so plugins now build with zig c++** (request: "check plugin native .so, coba compile dengan zig c++ biar bisa dibuat static build"). New `tools/build-native.sh` is the single entry point - it builds with **zig c++** and falls back to **g++** when zig is absent, and the three plugin `build.sh` files (`roliascan`, `madarascans`, `dummy`) are now thin wrappers over it, mirroring how `tools/build-wasm.sh` already handles TinyGo. **Size/dependency/toolchain:** roliascan 137,560 B (g++ 16.2.1) -> **290,416 B** (zig 0.18.0-dev.253+16d31ceed), madarascans 273,816 -> **572,688**, dummy 79,920 -> **256,768**. g++ links `libstdc++.so.6` + `libgcc_s.so.1` + `libm.so.6`; zig links its own libc++ and libgcc **statically**, so the artifact depends on **`libc.so.6` only** - that is the portability win (a host without a matching libstdc++ can still load it), at roughly 2x the file size. **Build time:** g++ 5.9 s cold and warm; zig 38 s cold but **74-87 ms warm** (its cache). **Full static is not reachable for a .so:** `zig c++ -static -shared` emits an **ar archive** ("current ar archive"), which `dlopen` cannot load - and a .so needs PIC relocations anyway; static *libc++* is the closest working target, which is what zig gives by default. (`g++ -static-libstdc++ -static-libgcc` also reaches libc-only, at 490,304 B - so g++ can do the same job, it just is not the default.) **The trap that cost a test failure:** `-fvisibility=hidden` is a per-plugin *contract*, not a global choice - `dummy/build.sh` deliberately omits it (its `extern "C"` block must stay exported as written) while `roliascan`/`madarascans` pass it and mark entry points with the `EXPORT` macro from `util.h`; the shared tool therefore takes the flag as an argument. With the same flags as `dummy/build.sh`, the zig build passes **5/5** native ABI tests. **Behaviour proven by A/B against the live site** (same host, same query): `search?q=one+piece` **byte-identical** (677 B), `detail/10724` **byte-identical** (1,089 B), `chapters/10724` 500 rows identical after normalising `released_at` - that field also differs between two consecutive fetches of the *same* g++ build (0/500 rows identical on that second comparison), so it is site-side per-response drift, not a build difference. The zig-built plugin was then deployed to `app_data` and loaded live: `plugin loaded (lazy) id=roliascan kind=go version=1`, search returned "One Piece" (id 10724) and "The Perks of Being a Villainess" (id 160380). `madarascans` returns `[]` under **both** builds - the site, not the toolchain. The toolchain is visible to the user: `built_with` is stamped per plugin and rendered as a badge on the plugins page, now reading `zig 0.18.0-dev.253+16d31ceed` instead of `g++ 16.2.1`. Added concept/native-zig-build.md.

## 2026-10-21

- **Every prod source file over 200 lines that held many functions is now split.** Request: "scan semua prod source cari file yang > 200 pecah jadi file kecil jika dalam 1 file terdiri dari banyak function jika 1 file terdiri dari 1 function besar biarkan saja. pecah file dengan cara move function ke file baru. sekalian optimasi dan bug check". 17 files split into 32 new ones across `internal/workers`, `internal/pluginmanager`, `internal/bridge`, `internal/httpserver`, `internal/hostnet`, `internal/database` and `cmd/goisekai-tray` - e.g. `enqueue.go` 346 -> 130/187/38, `goplug.go` 300 -> 164/53/97, `image.go` 286 -> 164/78/54, `verify.go` 261 -> 126/145. **No declaration was rewritten, only moved** with its doc comment; a mechanical splitter copied the package clause and import block, cut the declaration out, and `goimports` pruned the now-unused imports. Six prod Go files are still over 200 lines and were deliberately left (`views_library.go` 247, `views_manga.go` 222, `views_migrate.go` 212, `cmd/goisekai/main.go` 209, `database/schema.go` 206 - 0 functions, it is the ordered migration list - and `bridge/image_fetch.go` 201): each is dominated by one function. **No Lua template was split** - all nine over 200 lines are single-function modules (`one return function(data)`), and `views/detail.lua` already factors its three biggest blocks into partials. Added concept/prod-file-split-refactor.md with the rule, the per-file table and the two traps when cutting Go blocks by hand: (1) a block must END where the *next* declaration's doc comment starts, or the comment travels with the wrong function (the first splitter run did exactly that, putting `takeKey`'s comment on `fetchGate`); (2) the receiver is not the type - `pool.go` has two methods called `String`, so the splitter parses the receiver separately. Bug check: `staticcheck -checks 'SA*'` clean (only style noise; `ST1001` dot imports are intentional per AGENTS.md), `deadcode ./...` found exactly 2 unreachable funcs - `AutoSelectCandidate` (production-dead, no template or frontend references an auto-select, only two test files call it) and `hostSurfaceNames` (test-only: it is `helper_parity_test.go`'s source of truth, kept). One earlier claim is obsolete: `workers.Permanent()` is wired correctly (`enqueue.go` checks it with `errors.AsType[*ErrPermanent](err)`, `pool_test.go` has `TestPermanentNoRetry`). Gate: `go test ./... -count=1` exit 0, `go vet ./...` clean, `golangci-lint run` 0 issues, `gofmt -l internal/ cmd/` empty, `git diff --check` clean. `TestSamePriorityFIFO` failed once in a full `internal/bridge` run and passed 3/3 in isolation - the known timing flake, not a regression.

## 2026-10-20

- **Chapter lists came out oldest-first** (reported: "plugin weebcentral index
  manga chapter nya kebalik"). A library manga read 74, 73, 72 … while the
  same series read 1, 2, 3 … live — the order depended on library status
  because the cached path sorts `chapter_num DESC` and the live path returned
  the plugin's raw order. **Root cause: two plugins, not the host.** The repo's
  ABI is newest-first (the host's `chapterNeighbors`, `computeContinue` and
  `ListChaptersCached` all assume it; 15 plugins state it), but `weebcentral`
  and `comizy` reversed the site's newest-first fragment and shipped it
  ascending, each with a comment claiming the opposite ("the reader wants
  ascending"). An audit of all 34 installed plugins found only these two; 30
  return newest-first.
  - **Fix:** the reversal is gone from all four sources (Lua + Odin for both
    plugins) — the sites already answer in the ABI order. `chapter_num` was
    added where it was missing, because the cached path sorts by it and a
    plugin that ships none leaves the order to the `id` tie-break (reverse
    insertion order): `weebcentral` sent none, `comizy` sent 0 for every row.
  - **A false claim corrected:** comizy_odin's header said "reading progress is
    keyed off chapter_num, so inventing a number here would silently detach
    the reader". Progress is keyed off the chapter's **source id**
    (`GetChapterProgressForManga` joins `read_history` on `chapters.id`), so a
    real number detaches nothing.
  - **Split chapters:** the site labels a split chapter `Chapter 12-2` and a
    decimal one `Chapter 12.1`; `text.chapter_num` reads the leading number
    only, so both halves collapsed onto 12 (6 colliding rows in one manga).
    `comizy_chapter_num` folds the hyphen into the decimal the dot form uses.
  - **Trap:** `plugin_cache` is not invalidated by a plugin change — the
    chapter list lives 168 h, so the stale ascending JSON kept being served
    until it was purged.
  - **Verified live:** weebcentral 75 chapters `Chapter 74`→`Chapter 1`,
    unique 75/75; comizy 14 chapters, unique 14/14; DB after a re-sync holds
    14.2 / 14.1 / 13.2 / 13.1 / 12.2 / 12.1 / 11.1 / 10.0; both detail pages
    now render newest-first on the library *and* the live path.
  - **Test:** `internal/pluginmanager/weebcentral_chapter_order_test.go` — an
    offline fixture test for the Lua plugin plus live tests for both wasm
    plugins (descending order, non-zero and non-colliding numbers).
  - Full write-up: `okf-mcp/concept/chapter-order-newest-first-abi.md`.
- **Chapter upload dates showed the sync day** (reported: "tanggal upload nya
  chapter manga di website nya"). The chapter list has one date column,
  `fetched_at`, and the code used it for two different things. `UpsertChapter`
  wrote `CURRENT_TIMESTAMP` into it — in both INSERT branches and both
  `DO UPDATE` clauses — and never read the `c.FetchedAt` it had been handed,
  while the three read paths (`bridge/service.go:164`,
  `bridge/library_fallback.go:26` and `:99`) mapped that same column back out
  as `ReleasedAt`. The plugin's date was carried to the database and dropped on
  the way in. DB proof: `SELECT date(fetched_at), COUNT(*)` returned only sync
  days (2026-10-20: 11138, 10-19: 8702, 10-18: 2365, 10-17: 7889, …) — **zero**
  real upload dates were stored anywhere.
  - **Fix:** a dedicated `chapters.released_at` column (migration 30, NULL
    when the plugin knows no date), written from the plugin's date in both
    INSERT branches and both `DO UPDATE` clauses; `fetched_at` keeps meaning
    "when we synced". A reused column was rejected: 17 of the Lua plugins send
    no `released_at` at all, and a single column cannot express "no date
    known" next to a real one.
  - **The write path was probed, not assumed**: `TimestampExp(String(...))`
    and `TimestampExp(NULL)` were run against a real migrated DB first and
    round-trip exactly (text → `2024-09-07T17:04:15Z`, NULL → NULL).
  - **The jetgen trap, hit twice.** `cmd/jetgen`'s DDL list had fallen 7
    ALTERs behind `schema.go`'s migrations slice, so regenerating `.gen`
    produced a diff that *dropped* `CustomCover` from the mangas table (2
    insertions / 5 deletions) — a regression, not a fix. The missing ALTERs
    were added to the generator first; the regeneration then repaired the
    stale codegen as a side effect (`plugins.http_profile`, `plugins.genres`,
    `mangas.author`, `mangas.reader_prefs` came back). **Rule: before
    regenerating `.gen`, diff the generator's DDL against the migrations
    slice** — a generator that has fallen behind produces a diff that removes
    real columns.
  - Five regression tests across two levels (3 in `internal/database`, 2 in
    `internal/bridge`): the date survives the upsert round trip, a plugin with
    no date leaves the column NULL, a re-sync refreshes `fetched_at` without
    discarding `released_at`, and both read sites serve the site's own date.
  - **Live, on weebcentral after `POST /action/sync-manga`:** 75 chapters, 75
    with a date; histogram `2024-09-07 → 66`, `2024-11-11 → 9`; the detail
    page now reads "Sep 8, 2024" ×41 and "Nov 11, 2024" ×9, where it read
    "20 Oct 2026" on every row before. Timezone checked against the site's
    own dayjs formatter — both render in the visitor's local zone, so no
    day-shift is introduced. Detail:
    `concept/chapter-release-date-column.md`.
  - **Follow-up:** existing rows stay NULL until their manga is re-synced —
    there is no periodic library sync, only `POST /action/sync` and
    `/action/sync-manga/{pluginID}/{mangaID}`.

- **Weebcentral search covers were on the wrong titles** (reported: "title and
  cover shifted apart, searching dungeon"). Measured on the live
  `/search/data` response: every card renders **two** `<img>` (a desktop
  `article.hidden` and a mobile `article.lg:hidden` variant), but the plugin
  assumed one cover per card and zipped the cover list with the card list by
  position — so 31 of 32 covers belonged to the previous card. Fixed in both
  the Odin port and the Lua reference by matching the cover to its card on the
  ULID the cover URL carries (`/cover/fallback/<ULID>.jpg`) instead of on
  position. Live before 1/32, after 32/32; `348/348` cover URLs in the HAR
  contain a ULID, so the join is total. Detail:
  `concept/weebcentral-search-card-cover-alignment.md`.
  - **General rule recorded:** never zip two independently-extracted lists
    from a web page. Join them on an identifier the page itself provides. A
    positional zip is only safe when one element per record was measured.
  - **Same pattern not yet checked:** `mangakatana`, `mangataro`,
    `violetmanga`, `zeroscann` and `htmlscrape` all pair `titles[i]` with
    `covers[i]` positionally. Their selectors are narrower, but the count was
    never measured.
- **Installing a wasm plugin deadlocked the whole host** (found by the live
  test for the fix above, which hung for the full 5-minute timeout).
  `Install` held `m.mu` and `loadWasm` → `ensureWasmRuntime` took `m.mu`
  again; `sync.RWMutex` is not reentrant, so the second `Lock()` blocked
  forever and the process was stuck, not just the request. Pre-existing, not
  introduced by the info-script work: the wasm path was only ever reached
  through `Discover` → lazy load, which releases the lock before loading. The
  shared wazero runtime now has its own `runtimeMu`; lock order stays
  `mu -> runtimeMu`. Regression tests cover both `Install` and `LoadPlugin`.
  Detail: `concept/wasm-install-deadlock.md`.
  - **General rule recorded:** a lock taken by a helper must never be one its
    caller already holds. `m.mu` guards the plugin registry, `runtimeMu` the
    runtime, `challengeMu` the challenge map, `p.mu` one plugin.

- **Enrichment (info) scripts now run as Odin wasm**, and the plugins page lists
  them. The "info scripts can't be wasm" answer was wrong on three of four
  counts: the helper surface (`json`, `http`, `text`, `log`) was already complete
  and already dispatched by the wasm host. The real blocker was discovery, which
  built the entry as `main.lua` and hardcoded `kind: "lua"`. It now globs the
  folder and picks the entry by existence (`main.wasm` wins, matching plugin
  discovery), and an `info:` id requires `GetEnrichment` instead of the scraper
  ABI. Both installed scripts ported, source in `examples/info/*_wasm`; A/B
  against the Lua originals came back byte-identical on all six kinds
  (mangadex 11/1/4/3/22/9, mangaupdates 11/1/7/4/5/1).
  Detail: `concept/info-scripts-as-odin-wasm.md`.

- **Bug found in the mangaupdates Lua while A/B-ing:** it only ran
  `variantMatch(record.title)`, which needs 3+ tokens, and never compared
  `record.title` for exact equality (only `associated[]` got that check). So
  `"Solo Leveling"` — a title the API returns verbatim — yielded 0 items for
  every kind. The wasm port adds the exact comparison.

- **Fixed bulk "set this and before/after as read" marking the whole manga**
  (`internal/database/chapters_read.go`). `setChaptersToBound` filtered on
  `chapter_num` alone, but `comizy` ships **no chapter numbers**, so all 7 rows
  of that manga had `chapter_num = 0.0` and `chapter_num <= 0` (and `>= 0`)
  matched every row. The bound is now the pair `(chapter_num, id)` compared
  lexicographically, `id` being insertion order = source chapter order.
  Verified live on the exact manga from `manga_detail_action.har`: tick ch5 +
  "up to as read" → ch1–5 read / ch6–7 unread; tick ch5 + "down to as unread"
  → ch1–4 read / ch5–7 unread. 12 mangas in the reference DB have the all-zero
  shape. Regression: `TestSetChaptersBoundWithZeroChapterNums` (the old
  `TestSetChaptersBulkRead` used chapter_num 1..4, so it never caught this).
  Detail: `concept/chapter-bound-zero-chapter-num.md`.

- **Ported `mangaread` Lua → Odin wasm** (`examples/plugins/wasm/mangaread_odin/`,
  **413 244 bytes**). 264 LOC, the second Madara site after lhtranslation, so the
  known trap was re-verified rather than assumed: the chapter list only exists
  behind `POST /manga/<slug>/ajax/chapters/` (22 `wp-manga-chapter` in the AJAX
  response, none on the detail page).
  A/B with `plugin_cache` cleared: **search IDENTICAL (2483 B), detail IDENTICAL
  (767 B), pages IDENTICAL (3868 B)**; chapters 22/22 identical id+order with the
  one deliberate delta `manga_id` `""` → slug.
  **Search must be parsed per card block**, one per
  `div.row.c-tabs-item__content`, unlike lhtranslation's `<a title=...>` cards.
  Two real failures otherwise: the *first* occurrence of a slug on the page is the
  `<head>` `og:url`, so a ±3000-char window around the slug misses the card and
  the first result loses its cover; and an over-wide window lets the fallback
  regex steal a neighbouring card's cover. Genre paths also differ from
  lhtranslation — `/genres/<slug>/` with `/page/N/` paging, not
  `/manga-genre/<slug>/` with `?page=N`. `GetGenres` is the static 37-entry table,
  verified live via `/view/search` (cache NULL → **1369 B**).

- **Ported `mangakatana` Lua → Odin wasm** (`examples/plugins/wasm/mangakatana_odin/`,
  **409 315 bytes**). 265 LOC. **Cleanest port of the queue: all four endpoints
  byte-IDENTICAL to Lua with no deliberate delta** — search (2696 B), detail
  (749 B), pages (1197 B), chapters (1203 B), `plugin_cache` cleared.
  The id contract is load-bearing: manga id is `<slug>` (`naruto.1205`) and the
  chapter id is `<slug>:<cXXX>` (`naruto.1205:c700`), because the host pastes them
  into URL paths and a path segment cannot hold a slash.
  **Reader pages ship no usable image URLs**: every `<img>` has `data-src="#"` and
  JavaScript copies the real ones in; the list lives in a JS array literal
  (`var thzq=[...]`). The port takes the bracket literal with the most `http` URLs
  and ignores its name, so an obfuscated-variable rename cannot break it —
  verified live (37 image URLs lifted, `data-src` appears 38×).
  Selector gotcha worth remembering: the detail page's author/status cells have
  **combined** classes — `.value authors` and `.value status completed` — so
  `grep -c 'class="authors"'` returns 0 and looks like a broken selector.
  One adjustment: the description is read as the `<p>` in `div.summary` with
  `<br>` → newline, because `html.xpath_list_text(.../p/text())` used by the Lua is
  **not exposed to wasm** (no `xpath*` in `abi/`); both agree on the live markup
  (that paragraph carries zero `<br>`). Page numbering stays 1-based like Lua's
  `ipairs`. `GetGenres` is the static 27-entry table, verified live via
  `/view/search` (cache NULL → **987 B**). HAR moved to `tmp/mangakatana.har`.

- **Ported `lhtranslation` Lua → Odin wasm** (`examples/plugins/wasm/lhtranslation_odin/`,
  **415 460 bytes**). 258 LOC, WordPress + Madara theme, every read is a scrape
  (no JSON API). A/B with `plugin_cache` cleared: **search IDENTICAL (521 B),
  detail IDENTICAL (909 B)**; pages 33/33 url+order identical with `index`
  filled 0..32 (Lua omits the field, so the host stamps 0 on all 33); chapters
  4/4 id+order identical with `manga_id` filled and `released_at` present.
  **Madara trap, proven live:** the detail page carries **zero**
  `li.wp-manga-chapter` markup — the list only exists behind
  `POST /manga/<slug>/ajax/chapters/` with `X-Requested-With: xmlhttprequest`,
  so scraping the detail page silently yields nothing. Also: `img[data-src]`
  starts with tabs and newlines, and two title-ish blocks exist (the manga one is
  scoped by `div.post-title`).
  **Lua source bug found:** genres were sent as the URL-encoded display name, so
  `Gender Bender` → `/manga-genre/Gender%20Bender/` returns **404** live, while
  `gender-bender` returns 200. The port derives the slug (lower, hyphenated); the
  other 45 genres are unaffected. `GetGenres` verified live (cache NULL → 1672 B).
  Note on `released_at`: the newest chapter uses a relative site date ("20 hours
  ago"), so `text.date_to_iso` resolves it against the current clock and drifts
  between calls (15:56:54Z → 15:56:56Z) — site behaviour, not a regression;
  absolute-dated chapters match Lua exactly. HAR moved to `tmp/lhtranslation.har`.

- **Ported `mangataro` Lua → Odin wasm** (`examples/plugins/wasm/mangataro_odin/`,
  **410 403 bytes**). 223 LOC. The Lua header claims the chapter and page lists
  need `host.browser.evaluate` (a scroll-poll harvest) but the code never calls
  it — chapters come from a tokenised AJAX endpoint and pages from a plain
  content endpoint, so the port is pure HTTP.
  A/B with `plugin_cache` cleared: **search IDENTICAL (3331 B), detail IDENTICAL
  (415 B), pages IDENTICAL (6291 B)**; chapters **88/88** with identical id and
  order, and one deliberate delta: `released_at` is filled
  (`0001-01-01T00:00:00Z` → a real date) from the API's relative `date` field
  ("2 years ago") via `text.date_to_iso`, where Lua discards it.
  Chapter id is the bare numeric string (`"704814"`), which is exactly the
  `chapter_id` the page endpoint wants; reader URLs are reduced to trailing
  digits. The chapter request is signed client-side:
  `md5(unix_ts .. "mng_ch_" .. YYYYMMDDHH)[:16]`, UTC hour taken from
  `text.date_to_iso` (which formats UTC) rather than a local date call.
  `GetGenres` stays a **static 15-tag table** like the Lua (verified live: cache
  column NULL → **603 bytes**).

- **Host bug found and fixed: the WASI wall clock was frozen at 2022-01-01.**
  The first `mangataro` run returned `chapters = []` and the host logged a bare
  `403` with `_ts=1640995200` (= 2022-01-01). Root cause was **not** the plugin:
  wazero substitutes a fake clock when walltime is unconfigured
  (`internal/sys/sys.go:164` → `platform.NewFakeWalltime()`, whose
  `FakeEpochNanos` is `1640995200000*ms`, +1 ms per read). The host never called
  `WithSysWalltime()`, so **any** wasm plugin reading the clock got 2022.
  Fixed in `internal/pluginmanager/wasm.go` by adding `.WithSysWalltime()` to the
  module config; chapters went `3 B` → `16175 B`, identical to the Lua baseline.
  **Trap worth remembering:** a "build OK" probe for `time.now()` proves
  compilation only, never the runtime — the clock is only proven correct once a
  server accepts the signed request, and the symptom is a context-free `403`.

- **Ported `dynastyscan` Lua → Odin wasm** (`examples/plugins/wasm/dynastyscan_odin/`,
  **419 059 bytes**). 201 LOC but mostly JSON: `/series/<slug>.json`
  carries both the detail fields and the chapter list (`taggings`), and
  `/chapters/<permalink>.json` carries the pages, so only search and the genre
  index are HTML. A/B with `plugin_cache` cleared: **search IDENTICAL (1257 B),
  detail IDENTICAL (656 B), pages IDENTICAL (2023 B)**; chapters 32/32 with
  identical id+title+url and three deliberate deltas — `manga_id` filled (`""` →
  the slug), `released_at` filled from `taggings[].released_on` (Lua emits the
  zero time), and the two `chapter_num == 0` extras in feed order instead of
  Lua's arbitrary unstable-`table.sort` order (`stable_sort_by`).
  `GetGenres` was ported too and verified live: `/tags` yields **120 unique
  genres**, the cache column refilled from NULL to **5009 bytes**, and
  `/view/search?pluginID=dynastyscan` renders **162 `<option>`s**.
  **New trap, worth remembering:** `abi.find_all_pairs` decodes `[][]string`, but
  the host renders a **single**-capture row as a bare string (`wasmRegexRow` in
  `wasm_host_dispatch.go:144`), so a one-group pattern like `(?s)<dd>(.*?)</dd>`
  decoded to nothing **without an error** and search returned `[]`. Use
  `abi.find_all` for 1 capture group, `find_all_pairs` for 2+. **Second trap:**
  `plugins.genres = "[]"` means "no genre export, never call again", so a plugin
  that gains `GetGenres` stays dark until
  `UPDATE plugins SET genres=NULL WHERE id='…';`. `dynasty-scans.har` (143
  entries) confirmed the port's assumption: **0 XHR/fetch**, `/search?q=isekai`
  is a full server-rendered document, and `/tags/isekai` is hit — matching the
  archive path the port uses for genre-filtered searches. Lua original stays in
  `examples/plugins/lua/dynastyscan/`; only `app_data/plugins/dynastyscan/` was
  swapped (its `main.lua` moved aside, because the lua glob in `discovery.go`
  runs **before** `discoverWasm` and would otherwise win the id).

- **Swapped all seven Go/TinyGo wasm plugins for their existing C++ twins** in
  `app_data/plugins/` (komix_to, mangak, ksgroupscans, kunmanga_online,
  mangakakalot, manhawato, reimanga_net). **14 500 052 → 1 350 547 bytes, 10.7×**
  (reimanga_net 17.2×). Same plugin id and `plugin.json` `name` kept, so
  library/progress survives. `examples/` untouched — the Go sources stay as the
  permanent archive; only `app_data/` swaps. A/B (Go baseline vs installed C++)
  with `plugin_cache` cleared: search/detail/chapters **identical** on 6 of 7;
  `pages` goes `null` → `[]` everywhere, which the host decodes identically
  (`var result []types.Page`), so it is shape-only. `reimanga_net` is a **big
  win**: the Go build returned `plugin unreachable` for detail/chapters/pages,
  C++ returns real data (126 chapters incl. decimal `116.5`, author, cover,
  genres, 4 pages). `kunmanga_online` fixes a broken cover URL (missing slash →
  `onlinethumb/`). Full write-up: `concept/wasm-cpp-plugin-toolchain.md`.
- **Found the reason a C++ twin had gone stale: `build.sh` swallowed compile
  failures.** `clang++ … 2>&1 | grep -v "no version information available" ||
  true` makes the pipeline's status `grep`'s, and `|| true` discards it; when the
  compile fails the OLD `main.wasm` is still on disk, so `test -f main.wasm`
  passes and the script prints `exports OK (<size of the old binary>)`.
  `manhawato_cpp`'s `main.cpp` was from 10-19 11:31 while `main.wasm` was from
  10-17 11:24 — two days stale, reported as OK, and its live A/B then showed a
  missing `status` field. Fixed by `rm -f main.wasm` **before** the compile line
  in all eight `*_cpp/build.sh`, so a failed build now fails loudly. Rebuilt
  binaries are much smaller (komix_to 313 116 → 199 712 B) with unchanged flags
  and identical behavior — the old sizes were stale artifacts. **Never trust
  `exports OK` from a script that filters compiler output.**
- **Fixed a real C++ port bug found by A/B:** `mangakakalot_cpp` had
  `for (…; count < 48; …)`, a hardcoded cap that dropped the last 2 results (Go
  returned 50). Cap removed (Go has none; the host paginates) → 50/50 identical.
- **Operational gotcha:** `komix.to` can take ~16.7 s for one page, past the
  host's `invokeTimeout` (15 s, `internal/pluginmanager/manager.go:22`). The wasm
  module is then closed and the **next** call fails with
  `malloc failed for input` — a misleading message. Restarting the server
  restores it. On a wasm/C++ plugin, `malloc failed` means "module already
  closed", not "allocation failed".

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


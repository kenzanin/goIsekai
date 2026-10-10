# Handover — goIsekai (2026-10-10)

Handover doc for switching agent harness (pi → omp). Everything an agent needs to resume without the previous session's memory.

## 0. Project in one paragraph

goIsekai: manga library manager + reader, embedded HTTP server + browser UI, Go 1.27, module `goisekai`. Plugins (Lua/lunar, JS/goja, Go .so, Yaegi, wasm) fetch manga from external sites. Pure-Go SQLite (`CGO_ENABLED=0`), go-jet DSL queries, Lua HTML templates, Tailwind + Alpine frontend. Read `AGENTS.md` first — it has the full architecture map and command table.

## 1. Commands

| Task | Command |
|---|---|
| Build (Go only) | `just build` |
| Format + lint + tests (gate) | `just check` (0 issues = green) |
| Full tests | `just test` |
| Lua template lint | `just lint-lua` |
| Frontend format | `just fmt-web` (biome) |
| Regenerate brotli `.br` after any .js/.css edit | `just br` — **mandatory**, stale `.br` ships old code to the browser |
| Run server | `./goisekai` or via tray wrapper `./goisekai-tray` |

`just build` does **not** build `goisekai-tray` — build it manually with `go build -o goisekai-tray ./cmd/goisekai-tray`.

## 2. Runtime state (as of 2026-10-10 10:10 +0700)

- Tray PID 430999 → server PID 431009, port 8080, started 2026-10-10 10:10.
- Binary `./goisekai` built 2026-10-10 10:07 ≈ HEAD `635ed48`. Rebuild (`just build`) + restart tray after any Go change; Lua templates compile at startup (`hot_reload=false` in ini), so a restart is always needed after `internal/templates/` edits. Tray does **not** respawn its child — `POST /api/shutdown` kills the server only, then kill+relaunch the tray with `DISPLAY=:0.0 XDG_RUNTIME_DIR=/run/user/1000 DBUS_SESSION_BUS_ADDRESS=unix:path=/run/user/1000/bus`.
- **Onisaga cookies DEAD** (probe 09:47 → 403, age 230 min). Re-paste via the wizard before any live onisaga test. `cf_clearance` dies in <30 min — evidence: save 20:26 → 403 by 20:53; save 05:47 → 403 by 09:37.
- onisaga plugin (`app_data/plugins/onisaga/main.lua`, 222 lines, md5 `7a80117e`) is gitignored → backed up to `~/goisekai-backups/onisaga/` 2026-10-10.
- `cdp_engine = obscura` in `goisekai.ini` (obscura daemon usually dead; both CDP engines always time out 120s vs Cloudflare anyway — see `manual_cookies` below).
- `api_key` empty in ini — required for browser testing (reader.js same-origin fetches carry no `X-API-Key`).

## 3. Shipped this session (all on `main`, newest first)

| Commit | What |
|---|---|
| `635ed48` | pluginmanager: propagate invoke ctx into JS + wasm proxy calls (was `context.Background()`, unbounded CDP solve); `callJS` publishes ctx on `loadedPlugin.curCtx`, JS natives read via `Manager.invokeCtx`; wasm threads wazero ctx into `host_http_request` + `host_call`. Regression test pins it (1 s vs 8 s) |
| `202a021` | docs: add this handover |
| `5855d16` | css: wizard modal utility classes (`max-w-lg`, `border-neutral-600`, amber text, `p-2`) — were used by search.lua/detail.lua but missing from generated CSS |
| `9822609` | `manual_cookies=true` per-plugin meta → `solveAndSeed` returns instantly, no CDP ever; empty/stale cookies → wizard in ~340ms (was 247s) |
| `001e1ad` | Wizard cookie textarea autofocus |
| `555e561` | Challenge solves bounded by invoke ctx (15s) via lunar `frame.Context()`; `ErrChallenge` marker → typed `ChallengeError` re-raise in `callLua`; wizard re-raises instead of silent empty results |
| `7d20c14` | Startup re-seed of saved verify cookies into proxy jar |
| `dd79b90` | Save never blocks on plugin mutex (reads DB row directly); skip preemptive CDP solve when cookies exist; port-strip in cookie host match |
| `e656ac1` | Tray config from `goisekai.ini [tray]` (`server_bin`/`url`/`log_file`/`icon`, all optional → derive on empty) |
| `83e7045` | `pool.Shutdown` bounded at 5s — non-cooperative jobs (CDP solve) can't freeze SIGTERM |
| `420d140` | Cookie parser accepts Cookie-Editor JSON export |
| `423bed5` | `/api/shutdown`: flush response first, async trigger, `ErrServerClosed` → clean exit, PID file removed |
| `72d65f9`, `497511a`, `445813c` | Human-verify wizard gates (Go `viewSearch` + search.lua modal both consult `GetPluginVerifyState`) |

Key endpoints: `POST /api/shutdown` (200 JSON → clean exit, PID file removed), `POST /action/save-verify/{pluginID}` (303 back to origin).

## 4. Onisaga plugin — state ⚠️

**`app_data/` is gitignored** (`.gitignore:22`) → `app_data/plugins/onisaga/main.lua` exists **only on disk**. `git clean` or a fresh clone loses the whole rewrite. Either commit it with `git add -f` deliberately or back it up.

Rewritten this session against the site's Blade redesign, all ABI functions:

| ABI | Source | Status |
|---|---|---|
| `search_manga` | SSR cards: `<img alt="{Title} manga cover">` + overlay `<a aria-label>`; pattern bounded `.{0,700}` (coregex rejects repeats >~1000) | ✅ live, 24 cards |
| `get_manga_detail` | JSON-LD (type Book ×2) → title/desc/author; og:image cover; `/genre/` anchors for genres; status left empty (not on page) | ✅ live |
| `get_chapter_list` | Row anchors scoped by `wire:key="ch-..."` (Livewire rows only — unbounded `.*?` without it pairs the hero "Start reading" link with "Write a review" and drops Chapter 1); newest-first sort | ✅ offline 4/4 against saved HTML; **live re-check pending** |
| `get_page_list` | Inline Alpine JSON `pages: [{order, src}]` from reader HTML; URLs signed with `exp` ≈ 10 min | ⏳ **E2E pending** (blocked until fresh cookies) |

Plugin declares `needs_js`, `needs_human_verify`, `manual_cookies = true`.

## 5. Open items (next agent)

Done this session: #3 (cache purged), #4 (ctx propagation, `635ed48`), #6 (CSS committed), #7 (litter cleaned, minus HARs — see below).

Still open, **all three blocked on fresh cookies** (current cookies are 403-dead as of 09:37):

1. **Reader E2E** — the last blocked task. Re-paste cookies via the wizard first, then open `/view/read/onisaga/kaerazaru-hyouga/kaerazaru-hyouga:871798` and confirm pages render. Watch the signed `src` `exp` TTL (≈10 min): if the page opens after expiry, the image proxy must re-sign or re-fetch (refresh API needs a token; one probe hit 429 score-throttle).
2. **Chapters live re-check** — offline pattern proven (4/4), live check was blocked by expired cookies. `curl /view/manga/onisaga/kaerazaru-hyouga` → expect 4 rows with labels (0, 0.1, 0.2, 1), not "Write a review".
5. **`get_genres` export for onisaga** — site has `/genre/` pages; the search-page genre dropdown is empty because onisaga declares `genres = {}` with no export. Needs a live fetch to enumerate the genre slugs (saved HARs cannot help — see #7).

**Routine, not an item**: purge `plugin_cache` after any plugin edit — `DELETE FROM plugin_cache WHERE plugin_id='onisaga'`. GetMangaDetail 24h / GetChapterList 168h TTL masks fixes (symptom: old payload served, or a 13 ms detail = cache hit). Already purged 2026-10-10 09:45.

**HARs are useless — safe to delete.** The earlier note said keep them until reader E2E passes; that premise is wrong. `onisaga.har` (174 entries), `onisaga_search.har` (259), `cf_wizard.har` (10) all stored **no response bodies** (only `size`/`mimeType`, `text` empty) — a browser-export setting. `manga_action.har` has bodies but they are localhost `127.0.0.1:8080` captures of the atsumaru plugin, not onisaga. So they cannot serve issue #1/#2; they only add ~9 MB of untracked litter.

**Untracked leftovers not yet cleaned**: `.pi/` (prompts + skills from the previous pi harness — delete unless you still use it) and the openspec skill diffs (`.agents/skills/`, `.opencode/`) which are upstream tooling updates, not ours — review and commit separately.

## 6. Critical rules (distilled — these bite)

- **Never `CGO_ENABLED=1`** for builds; race tests only (`just race`).
- **State-changing POSTs** go on the `/action` sub-router (`r.Post`) — root-router POSTs bypass CSRF.
- **User-facing error text is host-owned**; plugins return error-code strings only, never messages.
- **URL-encode plugin-supplied IDs** in template URL paths — `h()` is HTML-escape only, chapter IDs contain `/`.
- **FTS**: every library mutation must `SyncFTS`/`SyncFTSTx`.
- **New ini settings** = first-class `Config` fields (`Save()` rewrites the whole file; must pass `TestSaveRoundTripsEveryField`).
- **Wizard visibility is two-layer**: Go `viewSearch` gate (`GetPluginVerifyState`) + search.lua modal condition — fix both or the modal redraws.
- **`Discover()` seeds zero-value plugin metadata** — view-time gates must load meta eagerly (self-load bridge call), not read stale `PluginMetas()`.
- **Plugin invocations run on `context.Background()+15s`**, not the job ctx — SIGTERM never reaches plugin code; shutdown relies on the 5s bounded drain.
- **JS invoke ctx lives on `loadedPlugin.curCtx`** — goja natives get no ctx from the runtime, so `callJS` publishes it and natives read it via `Manager.invokeCtx`. wasm threads its wazero call ctx into `host_http_request`/`host_call` instead. Without this the proxy call runs on `context.Background()` and outlives the deadline.
- **goja's `Interrupt` cannot abort a Go native already blocking inside it.** Only ctx tears the fetch down. And the interrupt goroutine must keep its original `time.After` select — switching it to watch `ctx.Done()` races with the deferred `cancel()` (which runs after `ClearInterrupt`) and can fire late, leaving a stale interrupt that fails the *next* call instantly. Symptom: `timeout` at 0.00s on the second call.
- **coregex** (not Go regexp) backs Lua `host.regex.*`: bounded repeats ≳1000 rejected silently → empty result.
- **Lua empty-table ABI**: results encode `{}`→`[]`, request headers stay `{}`; object ABI results must never return `{}`. Breaking this kills all Lua plugin HTTP silently.
- **lunar has no bitwise ops** (`~ & | << >>` parse-fail).
- **Verify cookie format**: parser accepts both `name=value; ...` and Cookie-Editor JSON array; pasting into templates is HTML-escaped, so JSON goes through `h()`-escaped textarea — parser sees `&#34;` only if template double-escapes (it doesn't — verified working).

## 7. Verification recipes

```bash
# restart cycle (graceful — also tests the shutdown API)
curl -s -X POST http://127.0.0.1:8080/api/shutdown; sleep 3
nohup ./goisekai > /tmp/goisekai.log 2>&1 &   # or restart the tray

# purge plugin cache after plugin edits
python3 -c "import sqlite3;c=sqlite3.connect('app_data/goisekai.db');c.execute(\"DELETE FROM plugin_cache WHERE plugin_id='onisaga'\");c.commit()"

# wizard gate states (expect: cookies+fresh → 200 results no modal; cookies dead → 200 + modal in <1s, log: 'blocked by challenge')
curl -s -m 30 "http://127.0.0.1:8080/view/search?pluginID=onisaga&q=isekai" | grep -c verify-modal

# direct site probe with saved cookies
python3 -c "import sqlite3,urllib.request;ck,ua=sqlite3.connect('app_data/goisekai.db').execute(\"SELECT cookies,user_agent FROM plugin_verify WHERE plugin_id='onisaga'\").fetchone();r=urllib.request.urlopen(urllib.request.Request('https://onisaga.com/',headers={'User-Agent':ua,'Cookie':ck}),timeout=15);print(r.status,r.read(200))"
```

Gate before any commit: `just check` (0 issues) + `just lint-lua` if templates touched + `.br` regen if frontend touched.

## 8. Conventions

- Commits: imperative summary, scope prefix (`wizard:`, `hostnet`, `tray:`…). Tracked code only; `app_data/` stays out of git unless explicitly `-f`.
- OpenSpec workflow lives in `openspec/` (proposal → apply → archive) — see AGENTS.md.
- Delegation: fixer lanes serial, `--auto` flag required; commit in-flight work before dispatching (lanes `git restore` on start).
- Language: replies to user in Indonesian (casual); code/comments/docs in English.

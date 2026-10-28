# Task: mangapill WASM plugin + genre UI + cover click enrichment decoupling

- generated: { by: "Reasonix agent", at: "2026-10-28T05:15:00+07:00" }

## Context
- Plugin language: **TinyGo WASM reactor** (`GOOS=wasip1 GOARCH=wasm`, `-target wasi -buildmode=c-shared`).
- TinyGo 0.43.0 @ `~/opt/tinygo/bin/tinygo`; Go toolchain pinned @ `~/sdk/go1.26.9` (GOTOOLCHAIN=local, CGO_ENABLED=0).
- Plugin source: `examples/plugins/wasm/mangapill/` → deploy `app_data/plugins/mangapill/`.
- Host regex engine: **coregex** (O(n), non-backtracking) — **no lazy quantifiers (`.*?`), no lookahead**. Every capture group must be its own single-group pattern.
- Repo: `/home/kenzanin/tmp/goIsekai` · Module `goisekai` · server entry `cmd/goisekai/main.go`, listens `127.0.0.1:8080`.

## Problem 1 — mangapill plugin
Create a TinyGo WASM plugin for mangapill.com so goIsekai can search, view details, list chapters, and read pages (derived from `mangapill.har`). No JSON API exists → HTML scraping.

### Key constraints / facts
- `Manga.ID` = numeric site id as string (e.g. `"2574"`); `Chapter.ID` = `"<mangaID>:<chapterNum>"` (e.g. `"2574:70"`); page arg = `Chapter.ID`.
- Cover CDN prefix: `https://cdn.readdetectiveconan.com/file/mangapill/i/`.
- Coregex: `.*?` fails → split multi-group scrape into 3 single-group `gAll` calls (id / cover / title), zip-by-index.
- `GetPageList` abi arg = full `"2574:1"` (NOT just mangaID).
- TinyGo `strings.Map` closure: `func(r rune) rune`; `return -1` = drop char. Must use `b[i]='-'` rune literal not `b[i]="-"` string.
- `text.chapter_num` hostcall → chapter number as float64 via JSON; `text.unescape` = `HTMLEntityUnescape`.
- `go.mod` → `go 1.26.9` (match TinyGo), `replace goisekai/pkg/types => ../../../../pkg/types`.
- Host `POST /api/sandbox/plugins/{id}/reload` hot-reloads wasm + metadata.

### Files
- `examples/plugins/wasm/mangapill/main.go` — Search/detail/chapters/pages/genres + ABI exports.
- `examples/plugins/wasm/mangapill/abi.go` — TinyGo malloc/free/allocCopy/pinned; readString; pack; reply.
- `examples/plugins/wasm/mangapill/hostcall.go` — hostCallRaw/hostCall/hostString/httpGet/httpPost/param/gAll/firstCap/log*.
- `examples/plugins/wasm/mangapill/text.go` — decodeString/parseTime/floatString/escapeGQL.
- `examples/plugins/wasm/mangapill/text_host.go` — chapterNum (hostcall + reNum fallback) / dateToISO / itoa / `reNum=\d+(?:\.\d+)?`.
- `examples/plugins/wasm/mangapill/go.mod`, `plugin.json`, `Makefile`.
- Logo: was duplicate of mangabuddy → replaced with SVG data-URI "MP" on `#0b79d0`. Removed `logo.png` (both examples/ and app_data/).

### Build & deploy
```sh
cd examples/plugins/wasm/mangapill
export GOROOT=~/sdk/go1.26.9 PATH="$GOROOT/bin:$HOME/opt/tinygo/bin:$PATH"
export GOTOOLCHAIN=local CGO_ENABLED=0
make build            # tinygo build -target wasi -buildmode=c-shared -o main.wasm . && wasm-opt -O4
mkdir -p app_data/plugins/mangapill && cp main.wasm plugin.json app_data/plugins/mangapill/
POST /api/sandbox/plugins/mangapill/reload
```
`wasm-objdump -x main.wasm` → exports: malloc, free, contract_version, _initialize, Search, GetMangaDetail, GetChapterList, GetPageList, GetGenres.

### Live smoke (server `127.0.0.1:8080`, 2026-10-28)
- `GET /api/sandbox/plugins/mangapill/search?q=dungeon` → 100 results (`id`/`title`/`cover_url`).
- `GET /api/sandbox/plugins/mangapill/detail/2574` → title, description, cover, 7 genres.
- `GET /api/sandbox/plugins/mangapill/chapters/2574` → 70 chapters `2574:70..2574:1`.
- `GET /api/sandbox/plugins/mangapill/pages/2574:1` → 49 jpg pages.

### Bugs hit + fixed
1. `b[i] = "-"` → `b[i] = '-'` (TinyGo rune type error).
2. Extra `}` in search() → `expected declaration, found 'return'`.
3. `mangaCardRe` w/ `.*?` matched 0 results → coregex has no lazy quantifier. Replaced multi-group pattern w/ 3 single-group `gAll` (id/cover/title), zip by index.
4. `mangaCoverRe = class="lazy[^"]*"` matched 0 → live HTML class is `"text-transparent lazy object-cover"` (lazy not first) + `data-src` precedes `class`. Fixed by anchoring CDN URL prefix directly.
5. `GetPageList` called with bare mangaID `"2574"` → route expects full `Chapter.ID`; used `2574:1`.
6. Logo cache stale `app_data/cache/images/plugins/mangapill/logo.png` (PNG) → `CachePluginLogo` not re-triggered by reload. Fix: `rm -rf app_data/cache/images/plugins/mangapill` + reload; `CachePluginLogo` lazy-re-cache on next `/view/plugins`.

### Plugin ABI note (GetGenres)
- Host `internal/pluginmanager/api.go:81` calls `m.call(p, types.GetGenresFunc, "{}")` — dispatcher wasm expects **every** export to accept `(ptr, n uint32)`.
- Initial plugin used `func GetGenres() uint64` → host call failed → `ListGenres` returns `nil` → DB cache `plugins.genres='[]'` stuck → genre dropdown never appears (mangadex-style `search.lua` renders dropdown only when `#genres > 0`).
- Fix: `func GetGenres(ptr, n uint32) uint64 { return reply(genresImpl()) }` (arg ignored; genres static). Matches rekan wasm manga_uno / manhawato / mangak / reimanga.
- After fix + reload + restart: DB `plugins.genres` populated w/ 28 genres; `/view/search?pluginID=mangapill` renders 29 `<option>` (All genres + 28).

---

## Problem 2 — Genre/tag selector: genre UI moved below search, multi-column checkbox grid

`views/search.lua` (NOT a `.br` asset — Lua templates rendered directly from disk, no brotli gate). Changes:

1. **Genre selector moved below search bar** — from inline `<select multiple size="8">` beside the search input, to its own `mt-4` block under the form: multi-column checkbox grid `grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6`, 28 checkboxes `input[type=checkbox] name="genre" value=<slug>`, label "Categories".
2. **Multi-select** — browsers submit each checked `name="genre"` as separate `?genre=a&genre=b`. JS history (search.lua inline script) updated to read `gEl.querySelectorAll('input[name="genre"]')` checkboxes (not the former `<select>.options`).
3. **Auto-hide** preserved — `genreBoxes ~= ""` guard; block renders only when selected plugin advertises genres (`#genres > 0`). For plugins w/o `get_genres` export, host returns nil → genres empty → no block.
4. Preselect works — `?genre=Action&genre=Fantasy` → those checkboxes `checked`.

### Build/serve
- Template edits only — no webpack; brotli gate doesn't apply to `.lua`.
- Clear stale genre cache + restart: after GetGenres signature fix, run `DELETE FROM plugins WHERE id='mangapill';` (or `handleRefreshPlugins` POST `/action/plugins/refresh`) then `POST /api/sandbox/plugins/mangapill/reload` + server restart so `ListGenres` re-fetches live and repopulates DB cache.

### Live verify (2026-10-28)
- `/view/search?pluginID=mangapill` → 28 checkboxes `name="genre"` (Action..Tragedy).
- `?genre=Action&genre=Fantasy` → Action + Fantasy pre-checked.
- DB `plugins.genres` length 1001 (json of 28).

---

## Problem 3 — Cover click must not trigger enrichment (decoupling)

### Root cause
`internal/templates/views/detail.lua`:
- Route: `GET /view/manga/{pluginID}/{mangaID}` → handler `viewMangaDetail` (`internal/httpserver/routes.go:45`).
- Template: `internal/templates/views/detail.lua` + partial `internal/templates/partials/detail_alt.lua`.
- **"Covers" button** (`detail.lua:107-116`, `148`) — form POST `/action/fetch-covers/...`, attr `data-open-covers="1"`. SPA handler `submitForm()` (`alpine-components.js:114`) opens `#cover-modal` (alt-cover selector) after fetch.
- **Enrichment button** (`detail_alt.lua:28`) — `id="enrichment-btn"`, collapses `#enrichment-body` (alt titles / summaries / categories). NOT related to cover image.

### The bug
`cmd/goisekai/frontend/lib/alpine-components.js:154` — `submitForm()` called `syncEnrichmentPanel()` **unconditionally** after *every* POST form submit, including `fetch-covers` ("Covers" button). So clicking "Covers" also fired the enrichment refresh → enrichment panel/content appeared alongside the alt-cover selector. (syncEnrichmentPanel defined as `() => {}` at line 902; called at init line 907.)

### Fix
Scope `syncEnrichmentPanel()` call to enrichment-only actions:
```js
// before (line 154):
if (window.syncEnrichmentPanel) syncEnrichmentPanel();

// after:
if (window.syncEnrichmentPanel && /fetch-enrichment|reset-enrichment/.test(action)) {
  syncEnrichmentPanel();
}
```
`action` is the form `action` attr captured earlier in `submitForm` (`var action = (form.getAttribute('action')||'').trim()`). fetch-covers action `/action/fetch-covers/...` does NOT match → enrichment not triggered.

### Serve / cache discipline (frontend)
- `cmd/goisekai/frontend/lib/alpine-components.js` is served via `brHandler` (serves the pre-compressed `.br` sibling when client accepts brotli); **no `.br` → stale bundle is served** (assets are unversioned URLs).
- **Mandatory after any JS/CSS edit: `brotli -q 11 -f -o ...js.br ...js`** (or `just br`). Test gate: `internal/httpserver/frontend_assets_guard_test.go::TestBrotliSiblingsMatchSource` fails otherwise.
- `.br` sibling size changed 9.5K→9.6K. After edit, restart server so the new bundle is served (brotli files read from disk; server already lazy-loads, restart confirms the fresh file is picked up).

### Verify (static + note on dynamic)
- Server restart (HTTP 200) after edit + `.br` rebuild.
- `curl /view/manga/mangapill/2574`: markup still has `data-open-covers="1"`=1 (open cover modal), `id="enrichment-btn"`=1 (enrichment button, separate), `id="cover-modal"`=1 (alt-cover picker). `grep -c syncEnrichmentPanel`<0> =0 (global call gone from rendered markup; JS now conditional).
- **Full dynamic verify** (click "Covers" → enrichment panel stays closed) requires a real browser (playwright-cdp / browser_act) — not loaded in this session. Static proof above is the deliverable.

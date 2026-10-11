---
type: Concept
title: Session dailies 2026-08-29 to 2026-09-30
description: Roll-up of 8 EchoesVault daily scratchpads — tls-client migration, human-verify/WebP ship, image burst pacing root cause, tailwind path bug, wasm host helpers and plugin ports, decimal-chapter audit, FanFox findings.
tags:
  - goisekai
  - daily
  - session-log
  - finding
generated:
  by: process:omp-assistant
  at: "2026-10-11T02:58:59Z"
---

# Session dailies 2026-08-29 → 2026-09-30

Consolidated roll-up of `EchoesVault/daily/` scratchpads (8 entries, dated
2026-08-29 through 2026-09-30). Facts sourced per date; not 1:1 — merged by
what they established.

## 2026-08-29 — hostnet proxy migrated to tls-client

- `internal/hostnet/proxy.go` moved from net/http `*http.Client` +
  `map[*cookiejar.Jar]` to lazily-created `map[string]tls_client.HttpClient`
  (one per plugin id, `sync.Mutex`-guarded), each with
  `WithTimeoutSeconds(30)`, `WithClientProfile(profiles.Chrome_146)`,
  `WithCookieJar(NewCookieJar())`; fhttp (`github.com/bogdanfinn/fhttp`)
  for request/response types.
- `buildHeaders` assembles the COMPLETE header set (defaults + sorted
  overrides, empty-Referer skipped) plus lowercase `HeaderOrderKey`, because
  tls-client fully replaces client defaults when `req.Header` is populated.
- Branch `omos/tlsclient`, commit `9bb18af`; tests/build/vet pass with
  `CGO_ENABLED=0`; go directive untouched at 1.27.

## 2026-08-31 — Human Verify v1 + WebP cache + image burst pacing

- **Human Verify v1 + WebP cache + per-plugin thumb_ratio** (`1929318`,
  24 files): cookie-paste verify panel in plugins page (open verify page /
  paste Cookie header / optional browser UA → seeded into tls-client cookie
  jar with UA override), challenge detection (403/503+marker) → amber
  banners, disk cache converts jpeg/png→.webp q85 (gen2brain/webp, pure
  Go), MangaDex emits thumb_ratio 0.703 with aspect-ratio slots.
- **Logs page copy fix + reader Retry re-fetch** (`1ee1670`): polling no
  longer destroys text selection (selectionchange guard, badge
  "Paused — selecting text"), `?limit=` 100–2000; reader Retry re-fetches
  page list preserving current page.
- **Image burst pacing — ROOT CAUSE of recurring 404** (`159d2df`): MD@Home
  nodes 404 concurrent/rapid bursts (NOT URL expiry; at-home does NOT rotate
  nodes). Browser draw+prefetch fires parallel /image requests → first 5
  "200"s were disk-cache hits, rest 404; client auto-retry amplified the
  rate limit ("kedap-kedip"). Fix in bridge GetImage: 1-slot semaphore +
  per-host pacing ≥1.1s (MD@Home convention) + 3x quadratic-backoff retry.
  Live-verified: 4 parallel fresh fetches → all 200, zero retries.
- **Favicon + CSS path bug** (`4778bdc`): tailwind regen had been writing to
  `frontend/static/lib/` which is NEVER served (routes.go strips `/static/`
  and serves `frontend/` directly) — live CSS stale since the mark-as-read
  arc. Fixed output path to `frontend/lib/tailwind.css`.
- Verified: disk cache serves cached page in 0.0009s with zero network
  fetches; MangaDex limits ~5 req/s per IP, `/at-home/server/{id}` 40
  req/min — 1.1s pacing safely under all.
- Gotchas: jet v6 has no runtime map `index()` → global helper
  `ratioAt(ratios, pluginID)`; jet UPDATE uses `col.EQ()` and
  `qrm.ErrNoRows` (not `sql.ErrNoRows`); jetgen struct collision → renamed
  `PluginVerifyRow`; sed `&` mangles URLs — build test URLs in python.

## 2026-09-07 — library sidebar

- `internal/templates/views/library.jet`: stats cards moved from top flex
  row into a left sidebar (`aside lg:w-56`) beside the manga grid; defensive
  duplicate-group toggle (`isset(.Duplicates) && .Duplicates > 0`
  guard; `<details>/<summary>` with member thumbnails); grid reduced to
  `xl:grid-cols-5`.

## 2026-09-10 — cover thumbnail bugs (template vs artifact)

- **Search page missing covers**: `internal/templates/views/search.lua`
  computed `coverHTML` (lines 39–52) but NEVER concatenated it into the
  result card (row build lines 67–79 emits only title + badge). Reference:
  `library.lua:257` appends `.. coverHTML`. Minimal fix: insert
  `.. coverHTML` before the `<div class="p-3">` at search.lua:72.
- **Updates page thumbnail sizing**: root cause was the Tailwind artifact,
  not the template. Commit `40ec3a1` overwrote compiled
  `cmd/goisekai/frontend/lib/tailwind.css` (35,664 B at `0374da9`) with the
  raw 37 B source directive and did not update `tailwind.css.br`; `make br`
  compressed the 37 B file, and `static_br.go` served it to brotli clients →
  zero utilities → intrinsic-size covers. Fix: keep `@tailwind` directives
  in a separate input file, regenerate the compiled sheet, `make br`, add a
  reproducible `make css` target.

## 2026-09-11 — plugin helper duplication audit

- 12 plugins (5 JS, 7 Lua), 30 source files. `enrich.js` byte-identical in
  js/{1manga,mangadex,mangzio}, near-copy in mangafire; `enrich.lua`
  byte-identical across all 6 real Lua plugins. `normalizeStatus` has 8
  diverging copies. Host natives already exist unused:
  `host.text.url_encode`, `host.text.html_decode`/`strip_html`,
  `host.http.get/post`. Top host-native candidates: normalizeStatus, enrich
  providers (~170 lines × 10 plugins), brace-balanced JSON extraction,
  epoch→RFC3339, query-string encoding with `+` space.

## 2026-09-23 — wasm host helpers + wasm plugin ports

- **Wasm host helpers (uniform with Lua/JS)**: generic
  `env.host_call(ptr,len)->i64` import in `internal/pluginmanager/wasm.go`;
  `wasm_host_helpers.go` dispatches JSON envelope
  `{"fn":"ns.name","args":[...]}` to the same pluginutil/htmldoc helpers
  Lua/JS natives call — text (15), codecs (9), crypto (6), json (2), html
  (9), regex (6; `gmatch` Lua-only), http (4 via `m.proxy`). New ABI const
  `types.HostCallFunc="host_call"`. Docs: `docs/plugin-wasm-helpers.md`.
  Decision: one dispatch import over ~50 per-helper imports. (Related:
  [wasm-plugin-local-helpers-decision](wasm-plugin-local-helpers-decision.md)
  for why hot paths stay local.)
- **Wasm plugin ports**: mangaka (Madara/AdminAjax,
  mangaka.cc) — toolchain is standard Go `GOOS=wasip1 GOARCH=wasm go build
  -buildmode=c-shared` (TinyGo unusable: 0.41.1 supports go ≤1.26, host
  1.27; plain build omits `//go:wasmexport`). madaradex (Madara/MangaPage
  mode, madaradex.org) — `/title/<slug>/` prefix, chapters parsed from the
  detail page, `warmAuth()` sync.Once fires `mdx_auth_refresh` once per
  process. ksgroupscans — archive cards put the thumbnail anchor `href`
  BEFORE its `<img>`, so "nearest preceding img" grabs the previous card's
  cover → per-card segment parser; chapters via `POST <mangaURL>/ajax/chapters/`.
- **mangak**: mangak.io is a Next.js frontend over a plain JSON API at
  `api.mangak.io` (gated by `Accept` + `Referer: https://mangak.io/`);
  endpoints `/titles/search`, `/titles/:hsid`, `/titles/:hsid/chapters`,
  `/titles/:hsid/chapters/:chapterId` (full `images[]`), `/genres`. ID
  scheme: Manga.ID = hsid, Chapter.ID = `<hsid>:<chapterId>`. Gotcha:
  reference `mangakakalot/main.go` calls `httpBody(url)` as fn — broken;
  correct convention `httpBody("http.get_body", u, headers)`.
- **mangakakalot search fix**: `ww2.mangakakalots.com` is dead/parked (app
  routes all 404 though homepage returns a cached 200 shell); switched
  siteURL to live mirror `https://www.mangakakalove.com`. `coverNear` root
  cause: new MangaBox markup nests `<img>` INSIDE the `/manga/<slug>` anchor
  → old logic returned previous card's cover (20/20 wrong); new logic exact
  `/manga/<slug>"` needle then prefer following img embedding the slug
  (avoids `solo-leveling` vs `solo-leveling-ragnarok` collisions) → 20/20
  search + 24/24 hot-manga covers correct.

## 2026-09-26 — decimal-chapter bug class + audits

- Commits: `559e08a` mangabuddy page-list regex missed decimal chapter dirs
  (`\d+` → `[\d.]+`; live 0 → 13 pages for chapter 7.1); `4fbd232`
  library-search partial keyword matching + ranking tier (title/alt 30–100,
  description ≤20, candidate set = FTS ∪ substring scan); `3400f73`
  valirscans same class (`/chapter/(\d+)` and id-split `^…:(\d+)$` →
  `[\d.]+`, raw token preserved so `chapter-7.1` ≠ `chapter-7` round-trips).
- Cross-plugin audit: only mangabuddy + valirscans affected by decimals;
  wasm (`\d+(?:\.\d+)?`), mangakatana (`[\d.]+`), 1manga/mangahub (`.+`),
  kaliscan (verbatim), Madara family (slug-based) were safe. Host-helper
  usage clean across Lua/JS — zero reimplementations found. Wasm keeps local
  helpers by decision (see
  [wasm-plugin-local-helpers-decision](wasm-plugin-local-helpers-decision.md)).
- Plugin cards page: per-plugin library manga count
  (`PluginView.LibraryCount` via `service.CountLibraryByPlugin()`,
  `plugin_cards.lua` "N manga in library" line).
- Dependency parity resolved as "already correct": wasm go.mod has only
  `require goisekai v0.0.0` + replace; coregex is a host security boundary
  for untrusted patterns, wasm stdlib regexp is RE2-safe; go-json is
  high-throughput marshaling, wasm does one decode per invoke.
- Next steps recorded: resume `wasm-manga-reader` OpenSpec change when
  requested (planning artifacts complete, implementation not started);
  optional e2e verify of valirscans decimal chapters on a real 7.x title.

## 2026-09-30 — FanFox Lua plugin

- Implemented FanFox Lua plugin (`examples/plugins/lua/fanfox/{main,util}.lua`
  + logo.png, installed to `app_data/plugins/fanfox/`), ABI
  contract_version 1, exports search_manga/get_manga_detail/get_chapter_list/
  get_page_list.
- Site findings: desktop search `fanfox.net/search?title=&page=N` works
  (mobile search JS-only); mobile detail has two `class="title"` divs (first
  = manga name, second = login prompt — first-match-wins is load-bearing);
  chapter rows wrap "Ch N" + &nbsp; + date span in one anchor; reader pages
  `/manga/{ch}/{N}.html` with tokenized img#image on zjcdn (protocol-relative
  `//` srcs; URLs expire — fetch at call time); pager `#chnav` shows a
  9-link window but `select.mangaread-page` lists all pages — take max
  `{N}.html` per chapter.
- Gotchas: fixture CRLF endings make line-based grep miss multi-line
  `href...>\s*text<` patterns (verify with `perl -0`); luacheck reports
  W111 for ABI globals (baseline mangafreak same, 0 errors);
  `get_manga_detail` HTTP failure must return `{id}` not `[]` (api.go
  obj-decoding comment).

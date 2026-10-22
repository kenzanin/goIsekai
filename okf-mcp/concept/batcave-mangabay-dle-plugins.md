---
id: batcave-mangabay-dle-plugins
title: BatCave and Manga-Bay plugins solve a custom DLE proof-of-work guard over plain HTTP
type: finding
tags: [goisekai, plugins, lua, batcave, mangabay, dle, proof-of-work, anti-bot, referer, slug-id, routing, genres]
---

# BatCave and Manga-Bay: a DLE guard solved without a browser

Request: *"buatkan plugin untuk batcave dan manga-bay"*, then *"buarkan plugin
untuk batcave.har referensi ada di ./extension-source/src/en/batcave"*. Both
sites run the same DataLifeEngine ("DLE") stack, so one guard module serves
both: `examples/plugins/lua/{batcave,mangabay}/dle.lua`.

Both plugins are five-function Lua sources — search, detail, chapter list, page
list, genres — and both now answer the full chain live.

## The guard is not Cloudflare, and that matters

The site serves a **custom** anti-bot guard, not a Cloudflare challenge. A plain
request returns HTTP **404** with a ~13 KB page carrying `pow_nonce` and
`token: "<base64>"`. The host's CDP solver keys off Cloudflare markers
(`turnstile`, `challenges.cloudflare.com`), so it does not recognise this one —
solving it in the browser was never needed, because the whole handshake is
plain HTTP:

1. `GET` target → 404 challenge page (`pow_nonce`, `token`).
2. Proof of work: find the smallest `n` where
   `sha256(token .. ":" .. n)` has hex prefix `"00"`.
3. `POST /_v` with a form field set (`token`, `mode=modern`, `workTime`,
   `iterations`, `hasCrypto`, `gms`, `fi`, `rtc`, `rtip`, `pow_nonce`,
   `pow_hash`, `webdriver`, `touch`, `screen_*`, `wgv`, `wgr`, `tz`, `dpr`,
   `cdp`, `cdpf`).
4. The response `Set-Cookie`s `__guard_trust` + `__guard_id`; the host's
   per-plugin cookie jar keeps them, so later requests in the session are clean.

`dle.get(site, path)` wraps this: fetch, `solve_guard` if the body carries
`pow_nonce`, then fetch once more and return the real body.

### `rtip` must be non-empty

`rtip` is the client IP the form reports. The server does not validate it, but an
**empty** value escalates the guard to Cloudflare Turnstile. A stable fake
(`1.2.3.4`) keeps the handshake on the cheap path. It is a `plugin.json` param
(`"rtip": "1.2.3.4"`) because a plugin may need to override it, and
`params_declared_test.go` enforces that every param a plugin reads is declared —
this one caught the omission.

### `timeout: 45`

A cold invokes pays the guard solve (a few hundred ms of sha256) plus a retry, which
exceeds the host's default **15 s** invoke budget. Both `plugin.json` files set
`"timeout": 45`.

## The manga id must not contain a slash

This was the one bug that looked like a parser failure and was not. Using the
full permalink (`https://batcave.biz/29706-thor-1966-2011.html`) as the manga id
made `/view/manga/batcave/…` answer **404** while every sandbox endpoint
answered 200.

The host routes `/view/manga/{pluginID}/{mangaID}` and
`/view/read/{pluginID}/{mangaID}/{chapterID}` as **fixed single segments**, and
the templates splice the id with `h()` — HTML escaping, **not** path escaping
(`internal/templates/views/search.lua`, `partials/detail_chapters.lua`). A `/` in
the id therefore adds path segments and the route stops matching. Every shipped
plugin already used a slash-free id (`comix/zgjz0-overwatch-undivided`,
`mangade/48847`, `mangabuddy/…ZDO0dQ`).

So the id is the DLE slug — `29706-thor-1966-2011.html`. `manga_id_of()` still
accepts a full permalink so an older stored id keeps working, and `path_of()`
turns either back into a site path for `dle.get` (which concatenates
`site .. path`, so an absolute URL would otherwise double the origin into
`https://sitehttps://site/...`).

Chapter ids *may* carry slashes — the templates escape those with `ue()`
(`url.PathEscape`), and `get_page_list` receives the raw string back, so the
chapter id is the reader path `/reader/<news_id>/<chapter_id>`, which holds both
halves the reader URL needs.

## The poster `<img>` is sometimes wrapped in a link

Batcave emits the cover two ways:

```html
<!-- Thor: direct child -->
<div class="page__poster img-wide"><img src="/uploads/posts/poster/6d/29706-….jpg"></div>

<!-- Batman: wrapped in a link ("Open full cover") -->
<div class="page__poster img-wide"><a href="/uploads/posts/hd/33758.webp"><img src="/uploads/posts/hd/33758.webp"></a></div>
```

A regex requiring `<img` immediately after the div matched only the first form, so
`cover_url` came back empty for a subset of titles while others worked — which
reads as intermittent rather than as a selector bug. Both plugins now try the
direct child, then the `<a>`-wrapped form.

## Manga-Bay's markup is not BatCave's

Same engine, different theme. BatCave's detail selectors match nothing on
Manga-Bay, so its parser is written against the real page:

| Field | Manga-Bay |
|---|---|
| Title | `<header class="page__header"><h1>…</h1>` (no `page__title-row` wrapper) |
| Alt titles | `<p class="page__alt">A; B; C</p>` — folded into the description |
| Cover | `<div class="page__poster">` → `<img src>` |
| Description | `<div class="page__desc-text full-text clearfix">` |
| Meta rows | `<dl class="page__list page__list--main"><div><dt>Status</dt><dd>Completed</dd></div>` |
| Genres | that same `<dl>`, `<dt>Genres</dt><dd>` one `<a>` per genre |
| Search cards | `<a href="…" class="cd" title="…">` + `<span class="cd__ph"><img data-src>` |

BatCave's search cards are `div.readed` with `<a href="…" class="readed__img …">`
— and the href comes **before** the class in the real markup, so a
class-then-href pattern matched zero cards.

## komacdn.com needs the site Referer with its trailing slash

Chapter images on Manga-Bay come from `komacdn.com`. Measured directly:

| Referer | Status |
|---|---|
| *(none)* | 403 |
| `https://komacdn.com/` (the CDN's own origin) | 403 |
| `https://manga-bay.biz` (**no** trailing slash) | 403 |
| `https://manga-bay.biz/` | **200** |

The host's image ladder falls back to `mgr.SiteURL(pluginID)`, which is the
`site_url` value verbatim — without the slash, so it 403s on every candidate.
The fix is the ABI's `Page.Headers`, not a host change: `get_page_list` emits
`headers = { Referer = BASE .. "/" }`, the same mechanism `mangadex`,
`topmanhua` and `madaradex` already use. The reader forwards it as the `referer`
query param (`reader.js` `imageUrl`) and `/image` turns it back into a header.

BatCave's own `img.batcave.biz` needs no Referer at all, so its pages stay bare.

## Genres are `{name, slug}`, not `{id, name}`

Both sites expose genres in a filter blob: BatCave's `window.__XFILTER__` and
Manga-Bay's `__XFILTER__` → `filter_items.g.values[{id, value}]`. The host's
genre ABI is `pluginmanager.Genre{Name, Slug}`.

Emitting `{id = v.id, name = v.value}` produced a **full** genre list that
rendered as `<option value="">Action</option>` — every slug empty, so browsing by
genre silently sent an empty filter. The slug is the DLE genre id, which is what
`search_manga` feeds back into `/ComicList/g=<id>/`:

```lua
out[#out + 1] = { name = v.value, slug = tostring(v.id) }
```

Verified live: BatCave 51 genres, Manga-Bay 159, each `<option value="1">Action`
etc.

## Two Lua traps worth recording

- **`host.regex.*` is RE2, not Lua patterns.** `(.-)` is a Lua lazy quantifier and
  fails to compile → `nil`. Use `(.*?)`. And `host.regex.find` returns `nil` on a
  miss, so every call needs `or ""`.
- **A `local function` must be declared before its first use.** Helpers placed
  *below* their call sites silently resolve to a nil global — the tell is
  luacheck's "unused function" warning. All helpers in both plugins are hoisted
  above the ABI functions.

## Verification

Live, against a running host:

| Check | BatCave | Manga-Bay |
|---|---|---|
| search | `thor` → 10+ cards | `dungeon` → 8 cards |
| detail | Thor (1966-2011), Stan Lee, 4 genres | Dungeon Meshi, Kui Ryouko, 10 genres |
| chapters | 396 | 163 |
| pages | `img.batcave.biz` jpg | `komacdn.com` webp, `Referer` header |
| `/view/manga` | 200 | 200 |
| `/view/read` | 200 | 200 |
| image proxy | 200 `image/webp` 191 KB | 200 `image/webp` 307 KB |
| cover proxy | 200 `image/avif` 21 KB | 200 `image/avif` 10 KB |
| genres | 51 | 159 |

`luacheck` 0 errors on both, `go test ./internal/pluginmanager/` ok (including
`TestEveryParamAReadPluginDeclares`), and the installed `app_data/plugins/<id>/`
copies match `examples/plugins/lua/<id>/` byte for byte.

## Operational notes

- **Clear the cache after changing plugin output**:
  `DELETE FROM plugin_cache WHERE plugin_id IN ('batcave','mangabay');` — genres
  are cached separately in `plugins.genres`, so a genre-shape change also needs
  `UPDATE plugins SET genres=NULL WHERE id IN (…);`.
- **Discovery runs at startup.** A `POST /api/sandbox/plugins/<id>/reload` only
  reaches an already-loaded plugin, so a brand-new folder needs a restart.
- **Two folders, not one shared module.** `app_data/plugins/` is a swap area: a
  Lua `main.lua` dropped into a folder that holds `main.wasm`/`main.so` makes Lua
  win discovery. Each plugin keeps its own copy of `dle.lua` rather than sharing
  one, to stay clear of that collision.

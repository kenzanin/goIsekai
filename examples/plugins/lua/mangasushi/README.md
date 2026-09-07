# mangasushi — Madara Theme Plugin

**Status: Installable — fully working.**

mangasushi.org uses WordPress + Madara theme. All four ABI surfaces work
over plain HTTP (GET search + POST chapter list; no Cloudflare, no JS
rendering needed).

## Correction (2026-09-07)

An earlier investigation concluded the site capped chapters at 5 per
series. That was wrong: the tested series simply had only 5 chapters.
POST `/manga/<slug>/ajax/chapters/` returns the FULL chapter list (e.g.
Lonely Attack on the Different World → 331 chapters). There is no
cap.

## How It Works

- **Search**: GET `/?s=<query>&post_type=wp-manga` → `.post-title h3 a`
  + `.summary_image img` cards. Note WP search only matches titles the
  site indexes (e.g. "isekai" → 9 results, "kodoku" → 0).
- **Detail**: GET `/manga/<slug>/` → `.post-content_item` label/value rows
  (status, author, genres, alt titles).
- **Chapters**: POST `/manga/<slug>/ajax/chapters/` (empty body,
  `X-Requested-With: XMLHttpRequest`) → `li.wp-manga-chapter` rows,
  newest-first, full list in one response (no pagination).
- **Pages**: GET `/manga/<slug>/chapter-<n>/` → `img.wp-manga-chapter-img`
  with the real URL in `data-src` (lazy-loaded; contains embedded
  tabs/newlines — the parser tolerates whitespace).


The Madara theme's `admin-ajax.php` endpoint — which is how the search
normally works — returns **0 bytes** for every query. This is not a
Cloudflare or transport issue: a real Chrome browser via CDP also gets 0
bytes from the same endpoint.

**Workaround used:** Server-rendered GET search (`/?s=query&post_type=wp-manga`)
works and returns results. The plugin uses this instead.

### 2. Chapter List Capped at 5 (Server-Side Limit)

The `/ajax/chapters/` endpoint returns only the 5 most recent chapters.
This is a **server-side cap** — the site itself enforces it.

**Confirmed:** The browser's own JavaScript makes the exact same POST to
`{base_url}ajax/chapters/` and also receives only 5 chapters. There is no
HTTP-based workaround. The Keiyoushi Tachiyomi Kotlin extension
(`ChapterMode.MangaAjax`) hits the same endpoint with the same limitation.

Without a full chapter list, the plugin is functionally incomplete for new
series — users can only access the 5 newest chapters.

## What Works

| Feature     | Status | Notes |
|-------------|--------|-------|
| Search      | ✅     | Via GET server-rendered search |
| Detail      | ✅     | Title, author, status, genres, description |
| Chapters    | ✅     | Full list, newest-first (331 for Lonely Attack) |
| Pages       | ✅     | Full page list per chapter |

## Installing

1. Copy `main.lua` (+ `logo.png` if present) to `app_data/plugins/mangasushi/`
2. Register/restart: the server discovers plugins from `app_data/plugins` at
   startup (`DB row` is created automatically)
3. Verify via sandbox: `/api/sandbox/plugins/mangasushi/search?q=isekai`

## Technical Details

- **Theme:** WordPress Madara (`ChapterMode.MangaAjax`)
- **Search:** GET `/?s={query}&post_type=wp-manga` (server-rendered HTML)
- **Chapters:** POST `{manga_path}/ajax/chapters/` (empty body + `X-Requested-With`)
- **Pages:** GET chapter URL, parse `img.wp-manga-chapter-img` `data-src`
- **Alt-titles:** MangaDex API (`alt_title_servers: [{id:"mangadex"}]`)

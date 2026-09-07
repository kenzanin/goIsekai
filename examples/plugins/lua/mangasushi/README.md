# mangasushi — Madara Theme Plugin (NOT INSTALLABLE)

**Status: Disabled — not installable as an active plugin.**

mangasushi.org uses WordPress + Madara theme. While the plugin code exists
here as a reference implementation, it cannot be used as an installed plugin
due to site-side limitations that cannot be worked around via HTTP.

## Why It Fails

### 1. AJAX Search Returns Empty (Site-Side Bug)

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
| Chapters    | ⚠️    | 5 most recent only (site cap) |
| Pages       | ✅     | Full page list per chapter |

## Reinstalling

If mangasushi.org changes their chapter-loading mechanism (e.g., adds
pagination or removes the 5-chapter cap), this plugin can be reactivated:

1. Copy `main.lua` to `app_data/plugins/mangasushi/main.lua`
2. `sqlite3 app_data/goisekai.db "UPDATE plugins SET is_active=1 WHERE id='mangasushi';"`
3. Restart the server

## Technical Details

- **Theme:** WordPress Madara (`ChapterMode.MangaAjax`)
- **Search:** GET `/?s={query}&post_type=wp-manga` (server-rendered HTML)
- **Chapters:** POST `{manga_path}/ajax/chapters/` (capped at 5)
- **Pages:** GET chapter URL, parse `div.page-break img`
- **Alt-titles:** MangaDex API (`alt_title_servers: [{id:"mangadex"}]`)

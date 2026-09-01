-- util.lua — KaliScan HTML parsing + HTTP helpers
-- Sibling module required by main.lua via require("util")

local util = {}

-- ─── URL encoding ──────────────────────────────────────────────────────────

function util.url_encode(s)
    return s:gsub("([^%w%-%.%_%~])", function(c)
        return string.format("%%%02X", string.byte(c))
    end)
end

-- ─── HTTP helper ───────────────────────────────────────────────────────────
-- Wraps http_request (host-provided global). Returns {status, headers, body}.
-- On error returns {status=0, error=...} per ABI contract.

function util.http_get(url, extra_headers)
    local req = { url = url, method = "GET", headers = {} }
    if extra_headers then
        for k, v in pairs(extra_headers) do
            req.headers[k] = v
        end
    end
    return http_request(req)
end

-- ─── Search result parsing ─────────────────────────────────────────────────
-- Parses the /search?q=...&page=N HTML page.
--
-- Structure per result (book-item):
--   <a title="TITLE" href="/manga/SLUG">
--     <img ... data-src="COVER_URL" ...>
--   </a>
--
-- Pagination: <div class="paginator"> has a <select> whose last
-- <option> contains the max page number.

function util.parse_search(html)
    local results = {}

    -- Total pages from the paginator <select> — last <option value="/search?page=N&q=...">
    local max_page = 1
    local last_page_num = string.match(html,
        '<option%s+value="/search%?page=(%d+)[^"]*">(%d+)</option>%s*</select>')
    if last_page_num then
        max_page = tonumber(last_page_num) or 1
    end

    -- Extract manga entries: each <a> linking to /manga/SLUG with a cover img inside.
    -- Pattern: <a title="TITLE" href="/manga/SLUG" ... > ... data-src="COVER"
    -- We iterate all such pairs; book-item links are the ones with data-src covers.
    local seen = {}
    -- Site emits TWO anchors per manga (cover anchor + title anchor). Only
    -- the cover anchor has <img ... data-src> directly inside it, so we match
    -- title/href/cover in ONE anchor (img must follow the opening tag) — the
    -- old lazy `.-data%-src` crossed `</a>` into the NEXT manga's cover and
    -- shifted every thumbnail by one card.
    for title, slug, cover in string.gmatch(html,
        '<a%s+title="([^"]-)"[^>]*href="/manga/([^"]-)"[^>]*>%s*<img[^>]-data%-src="([^"]-)"')
    do
        if not seen[slug] then
            seen[slug] = true
            table.insert(results, {
                id = slug,
                title = title,
                cover_url = cover
            })
        end
    end

    return {
        total = max_page,
        results = results
    }
end

-- ─── Manga detail parsing ─────────────────────────────────────────────────
-- Parses /manga/SLUG HTML page.
--
-- Sections:
--   <h1>TITLE</h1>                                    in <div class="name box">
--   Authors :</strong> <a ...><span>AUTHOR</span>     in meta box
--   Status :</strong> <a ...><span>STATUS</span>
--   Cover: <div class="cover"> ... <img data-src="URL">
--   Description: <p class="content" ...>TEXT</p>      in summary tab panel
--   Genres: <a href="/genres/G/">G</a>

function util.parse_manga_detail(html, manga_id)
    local detail = { id = manga_id }

    -- Title: <h1>TITLE</h1>
    detail.title = string.match(html, '<h1>([^<]+)</h1>') or ""

    -- Author: Authors :</strong> <a ...><span>AUTHOR</span>
    detail.author = string.match(html,
        'Authors[^<]*</strong>%s*\n?%s*<a[^>]*>%s*<span>([^<]+)</span>') or ""

    -- Status
    detail.status = string.match(html,
        'Status[^<]*</strong>%s*\n?%s*<a[^>]*>%s*<span>([^<]+)</span>') or ""

    -- Cover: data-src inside the cover div
    detail.cover_url = string.match(html,
        'class="cover[^"]*"[^>]*>.-<img[^>]*data%-src="([^"]-)"') or ""

    -- Description: <p class="content" ...>TEXT</p>
    local desc = string.match(html, '<p class="content"[^>]*>(.-)</p>') or ""
    -- Strip inner HTML tags
    desc = desc:gsub("<[^>]+>", ""):gsub("^%s+", ""):gsub("%s+$", "")
    detail.description = desc

    -- Genres
    local genres = {}
    for _, g in string.gmatch(html, 'href="/genres/([^/"]+)/"[^>]*>%s*([^<]+)%s*') do
        genres[#genres + 1] = g
    end
    detail.genres = genres

    return detail
end

-- ─── Chapter list parsing ──────────────────────────────────────────────────
-- Parses /manga/SLUG HTML for the full chapter list.
--
-- Each chapter item:
--   <li id="c-NUMBER">
--     <a href="/manga/SLUG/chapter-NUMBER" title="...">
--       <div>
--         <strong class="chapter-title">TITLE</strong>
--         <time class="chapter-update">UPLOAD_TIME</time>

function util.parse_chapter_list(html, manga_id)
    local chapters = {}

    for number, href, title, upload_time in string.gmatch(html,
        'id="c%-([^"]-)"[^>]*>%s*' ..
        '<a[^>]*href="([^"]-)"[^>]*title="([^"]-)"[^>]*>%s*' ..
        '<div>%s*<strong class="chapter%-title">([^<]-)</strong>%s*' ..
        '<time[^>]*>([^<]*)</time>')
    do
        -- chapter_id: path after /manga/ with "/" swapped for ":" (e.g.
        -- "SLUG:chapter-98") — host routing treats the ID as one opaque
        -- segment, so a raw slash would 404 the reader URL.
        local chapter_id = string.match(href, '/manga/(.+)')
        if not chapter_id then
            chapter_id = manga_id .. "/chapter-" .. number
        end
        chapter_id = chapter_id:gsub("/", ":")

        chapters[#chapters + 1] = {
            id = chapter_id,
            manga_id = manga_id,
            chapter_num = tonumber(number) or 0,
            title = title
        }
        -- ponytail: uploaded_at (human-relative, e.g. "2 days ago") dropped:
        -- types.Chapter wants RFC3339 time; add a parser when sorting by date matters.
    end

    return chapters
end

-- ─── Page list parsing ─────────────────────────────────────────────────────
-- Parses the chapterServer HTML response.
-- Each page image: <div ... data-src="URL" ...>
--
-- TODO: verify against live site — the data-src extraction needs
-- confirmation that chapterServer returns this exact shape.

function util.parse_page_list(html, referer)
    local pages = {}
    local n = 0
    for url in string.gmatch(html, 'data%-src="([^"]-)"') do
        -- Skip static assets / loading placeholders
        if not string.match(url, '%.svg$') and not string.match(url, '/static/') then
            n = n + 1
            pages[n] = {
                index = n - 1,
                url = url,
                headers = { ["Referer"] = referer } -- image CDN needs the chapter page as Referer
            }
        end
    end
    return pages
end

return util

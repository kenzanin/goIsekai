-- LHTranslation plugin for goIsekai (Lua / Lunar VM)
-- Site: https://lhtranslation.net (WordPress Madara theme)
-- ABI contract version: 1

PLUGIN = {
    contract_version = 1,
    name = "LHTranslation",
    site_url = "https://lhtranslation.net",
    logo = "logo.png",
    verify_url = "https://lhtranslation.net",
    needs_human_verify = false,
    thumb_ratio = 0.703,
}

BASE = "https://lhtranslation.net"

-- ─── helpers ────────────────────────────────────────────────────────────────

local trim = host.text.trim
local unescape = host.text.unescape

-- ─── ABI: search_manga ──────────────────────────────────────────────────────
-- arg: {"query":"...","page":1}. Madara search: /?s=Q&post_type=wp-manga
-- Single page per call (Madara caps results page); return-all from this page.

function search_manga(arg)
    local args = host.json.decode(arg)
    local query = args.query or ""
    local page = args.page or 1

    local url = BASE .. "/?s=" .. host.text.url_encode(query) .. "&post_type=wp-manga"
    if page > 1 then
        url = url .. "&paged=" .. tostring(page)
    end
    local body = host.http.get_body(url)
    if not body then return host.json.encode({}) end

    local results = {}
    -- Parse per item: anchor <a href=".../manga/SLUG/" title="..."> followed
    -- within the same block by <img data-src="COVER">. Per-block parsing keeps
    -- covers aligned with slugs even when other /manga/ anchors intervene.
    local pos = 1
    while true do
        local s = host.regex.find_index(body, [[<a\s+href="]], pos)
        if not s then break end
        local e = host.regex.find_index(body, "</a>", s)
        if not e then break end
        local block = body:sub(s, e)
        local href, title = host.regex.find(block, [[^<a\s+href="([^"]*/manga/[^"]+)"\s+title="([^"]*)"]])
        if href then
            local slug = host.regex.find(href, [[/manga/([^/]+)/$]])
            if slug then
                local cover = host.regex.find(block, [[data-src="([^"]+)"]]) or host.regex.find(block, [[\ssrc="([^"]+)"]]) or ""
                if host.regex.match(cover, "dflazy") then cover = "" end
                results[#results + 1] = {
                    id = slug,
                    title = unescape(title),
                    cover_url = trim(cover)
                }
            end
        end
        pos = e + 4
    end

    log.debug("lhtranslation search q=" .. query .. " page=" .. tostring(page) .. " found=" .. tostring(#results))
    return host.json.encode(results)
end

-- ─── ABI: get_manga_detail ──────────────────────────────────────────────────
-- arg: '"slug"'. Detail page fields: summary_image cover, post-title h3/h1,
-- author-content, artist-content, genres-content, post-status, summary__content.

function get_manga_detail(arg)
    local slug = host.json.decode(arg)
    local body = host.http.get_body(BASE .. "/manga/" .. slug .. "/")
    -- empty table encodes as []; detail must stay an OBJECT — emit {id} only
    if not body then return host.json.encode({id = slug}) end

    local detail = { id = slug, title = "", author = "", description = "",
        cover_url = "", genres = {}, status = "" }

    -- cover: <div class="summary_image"> <a href> <img data-src="...">
    local cover_block = host.regex.find(body, [[(?s)class="summary_image".*?</a>]]) or ""
    detail.cover_url = trim(host.regex.find(cover_block, [[data-src="([^"]+)"]]) or host.regex.find(cover_block, [[\ssrc="([^"]+)"]]) or "")

    -- title: first <h1..> inside post-title h1 variant, else <title>
    local tblock = host.regex.find(body, [[(?s)class="post-title[^"]*">(.*?)</div>]]) or ""
    detail.title = unescape(trim(host.regex.find(tblock, [[(?s)<h[1-6][^>]*>(.*?)</h[1-6]>]]) or ""))

    -- author + artist
    local ablock = host.regex.find(body, [[(?s)class="author-content">(.*?)</div>]]) or ""
    detail.author = host.text.strip_html(ablock):gsub(",", ", ")

    -- genres
    local gblock = host.regex.find(body, [[(?s)class="genres-content">(.*?)</div>]]) or ""
    for g in host.regex.gmatch(gblock, [[<a[^>]*>([^<]+)</a>]]) do
        local name = unescape(trim(g))
        if name ~= "" then detail.genres[#detail.genres + 1] = name end
    end

    -- status: post-status block, second summary-content (after Release)
    local sblock = host.regex.find(body, [[(?s)class="post-status">(.*?)$]]) or ""
    if sblock ~= "" then
        -- find the "Status" heading then the next summary-content
        local after = host.regex.find(sblock, [[(?s)Status\s*</h5>.*?class="summary-content">\s*([^<]+)]])
        if after then
            detail.status = host.text.normalize_status(trim(after))
        end
    end

    -- description: summary__content block, strip tags (may contain nested divs —
    -- capture to the "show-more" span boundary)
    local dblock = host.regex.find(body, [[(?s)class="summary__content[^"]*">(.*?)<span\s+class="[^"]*content-readmore"]]) or ""
    if dblock == "" then dblock = host.regex.find(body, [[(?s)class="summary__content[^"]*">(.*?)</div>]]) or "" end
    detail.description = host.regex.replace(host.text.strip_html(dblock), [[\s+]], " ")

    return host.json.encode(detail)
end

-- ─── ABI: get_chapter_list ──────────────────────────────────────────────────
-- arg: '"slug"'. Madara loads chapters via POST {base}/manga/SLUG/ajax/chapters/
-- Returns newest-first: /ajax/chapters/ returns newest first already.

function get_chapter_list(arg)
    local slug = host.json.decode(arg)
    local body = host.http.post_body(BASE .. "/manga/" .. slug .. "/ajax/chapters/", "",
        { ["X-Requested-With"] = "xmlhttprequest" })
    if not body then return host.json.encode({}) end

    local chapters = {}
    for li in host.regex.gmatch(body, [[(?s)<li\s+class="wp-manga-chapter[^>]*>(.*?)</li>]]) do
        local href = host.regex.find(li, [[href="([^"]+)"]])
        local label = trim(host.regex.find(li, [[>([^<]*[Cc]hapter[^<]*)<]]) or "")
        local date = trim(host.regex.find(li, [[chapter-release-date[^>]*>\s*<i[^>]*>([^<]+)</i>]]) or host.regex.find(li, [[chapter-release-date[^>]*>\s*<a[^>]*>([^<]+)</a>]]) or "")
        if href and label ~= "" then
            local cid = host.regex.find(href, [[/manga/[^/]+/([^/]+)/$]])
            -- Madara emits "July 7, 2026"; the host knows the layouts sites use
            -- and returns "" for the rest, which leaves released_at out.
            local iso = host.text.date_to_iso(date)
            local ch = {
                id = slug .. ":" .. (cid or label),
                chapter_num = host.text.chapter_num(label),
                title = label,
                url = href
            }
            if iso ~= "" then ch.released_at = iso end
            chapters[#chapters + 1] = ch
        end
    end

    log.debug("lhtranslation chapters slug=" .. slug .. " count=" .. tostring(#chapters))
    return host.json.encode(chapters)
end

-- ─── ABI: get_page_list ─────────────────────────────────────────────────────
-- arg: '"slug:chapter-N"'. Chapter page: <div class="reading-content"> with
-- <img data-src="..."> inside .page-break divs.

function get_page_list(arg)
    local path = host.json.decode(arg) -- "slug:chapter-N"
    path = path:gsub(":", "/")
    local body = host.http.get_body(BASE .. "/manga/" .. path .. "/")
    if not body then return host.json.encode({}) end

    local pages = {}
    -- Find all <img ...> tags carrying wp-manga-chapter-img, pull data-src
    local pos = 1
    while true do
        local s = host.regex.find_index(body, "<img", pos)
        if not s then break end
        local e = host.regex.find_index(body, ">", s)
        if not e then break end
        local tag = body:sub(s, e)
        if host.regex.match(tag, "wp-manga-chapter-img") then
            local src = host.regex.find(tag, [[data-src="([^"]+)"]]) or host.regex.find(tag, [[\ssrc="([^"]+)"]])
            if src then pages[#pages + 1] = { url = trim(src) } end
        end
        pos = e + 1
    end

    log.debug("lhtranslation pages " .. path .. " count=" .. tostring(#pages))
    return host.json.encode(pages)
end


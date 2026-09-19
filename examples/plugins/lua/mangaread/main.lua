-- MangaRead plugin for goIsekai (Lua / Lunar VM)
-- Site: https://www.mangaread.org (WordPress Madara theme)
-- ABI contract version: 1

PLUGIN = {
    contract_version = 1,
    name = "MangaRead",
    site_url = "https://www.mangaread.org",
    logo = "logo.png",
    verify_url = "https://www.mangaread.org",
    needs_human_verify = false,
    thumb_ratio = 0.703,
}

BASE = "https://www.mangaread.org"

-- ─── helpers ────────────────────────────────────────────────────────────────

local trim = host.text.trim
local unescape = host.text.unescape

-- ─── ABI: search_manga ──────────────────────────────────────────────────────

function search_manga(arg)
    local args = host.json.decode(arg)
    local query = args.query or ""
    local page = args.page or 1

    -- Madara GET search: admin-ajax.php returns 0 bytes, use server-rendered GET instead
    local url = BASE .. "/?s=" .. host.text.url_encode(query) .. "&post_type=wp-manga"
    local body = host.http.get_body(url)
    if not body or body == "" then
        log.error("mangaread search empty response")
        return host.json.encode({})
    end

    local results = {}
    local seen = {}

    -- Parse .post-title > h3 > a for title + href
    -- Structure: <div class="post-title"><h3 class="h4"><a href="URL">TITLE</a></h3></div>
    for href, title in host.regex.gmatch(body, [[(?s)post-title[^>]*>.*?<a[^>]*href="([^"]+)"[^>]*>([^<]+)</a>]]) do
        local slug = host.regex.find(href, [[/manga/([^/]+)/]]) or host.regex.find(href, [[/manga/([^/]+)$]])
        if slug and not seen[slug] then
            seen[slug] = true
            -- Find cover: search nearby for data-src or src with image URL
            local cover = ""
            local pos = host.regex.find_index(body, host.regex.quote(slug), 1)
            if pos then
                local area = body:sub(math.max(1, pos - 3000), math.min(#body, pos + 3000))
                -- Madara lazy images: data-src with possible tabs/newlines before URL
                cover = host.regex.find(area, [[data-src="[\t\n\s]*(https?://[^"]+)"]])
                    or host.regex.find(area, [[data-src="(https?://[^"]+)"]])
                    or host.regex.find(area, [[src="(https?://[^"]+)"]])
                if cover then cover = trim(cover) end
            end
            results[#results + 1] = {
                id = slug,
                title = unescape(title),
                cover_url = cover or ""
            }
        end
    end

    log.debug("mangaread search q=" .. query .. " page=" .. tostring(page) .. " found=" .. tostring(#results))
    return host.json.encode(results)
end

-- ─── ABI: get_manga_detail ──────────────────────────────────────────────────

function get_manga_detail(arg)
    local slug = host.json.decode(arg)
    local body = host.http.get_body(BASE .. "/manga/" .. slug .. "/")
    if not body then return host.json.encode({id = slug}) end

    local detail = { id = slug, title = "", author = "", description = "",
        cover_url = "", genres = {}, status = "" }

    -- cover
    local cover_block = host.regex.find(body, [[(?s)class="summary_image".*?</a>]]) or ""
    detail.cover_url = trim(host.regex.find(cover_block, [[data-src="([^"]+)"]]) or host.regex.find(cover_block, [[src="([^"]+)"]]) or "")

    -- title
    local tblock = host.regex.find(body, [[(?s)class="post-title[^"]*">(.*?)</div>]]) or ""
    detail.title = unescape(trim(host.regex.find(tblock, [[(?s)<h[1-6][^>]*>(.*?)</h[1-6]>]]) or ""))

    -- author
    local ablock = host.regex.find(body, [[(?s)class="author-content">(.*?)</div>]]) or ""
    detail.author = host.text.strip_html(ablock):gsub(",", ", ")

    -- genres
    local gblock = host.regex.find(body, [[(?s)class="genres-content">(.*?)</div>]]) or ""
    for g in host.regex.gmatch(gblock, [[<a[^>]*>([^<]+)</a>]]) do
        local name = unescape(trim(g))
        if name ~= "" then detail.genres[#detail.genres + 1] = name end
    end

    -- status
    local sblock = host.regex.find(body, [[(?s)class="post-status">(.*?)$]]) or ""
    if sblock ~= "" then
        local after = host.regex.find(sblock, [[(?s)Status\s*</h5>.*?class="summary-content">\s*([^<]+)]])
        if after then
            detail.status = host.text.normalize_status(trim(after))
        end
    end

    -- description
    local dblock = host.regex.find(body, [[(?s)class="summary__content[^"]*">(.*?)<span\s+class="[^"]*content-readmore"]]) or ""
    if dblock == "" then dblock = host.regex.find(body, [[(?s)class="summary__content[^"]*">(.*?)</div>]]) or "" end
    detail.description = host.regex.replace(host.text.strip_html(dblock), [[\s+]], " ")

    return host.json.encode(detail)
end

-- ─── ABI: get_chapter_list ──────────────────────────────────────────────────
-- Madara chapters via POST {base}/manga/SLUG/ajax/chapters/

function get_chapter_list(arg)
    local slug = host.json.decode(arg)
    local body = host.http.post_body(BASE .. "/manga/" .. slug .. "/ajax/chapters/", "", {
        ["X-Requested-With"] = "xmlhttprequest",
        ["Content-Type"] = "application/x-www-form-urlencoded",
    })
    if not body then return host.json.encode({}) end

    local chapters = {}
    local seen = {}

    -- Madara chapter list: <li class="wp-manga-chapter"><a href="URL">LABEL</a>...<i>DATE</i></li>
    for li in host.regex.gmatch(body, [[(?s)<li[^>]*class="wp-manga-chapter[^"]*"[^>]*>(.*?)</li>]]) do
        local href = host.regex.find(li, [[href="([^"]+)"]])
        local label = trim(host.regex.find(li, [[>([^<]*[Cc]hapter[^<]*)<]]) or "")
        local date = trim(host.regex.find(li, [[(?s)chapter-release-date[^>]*>.*?<i>([^<]+)</i>]]) or "")
        if href and label ~= "" then
            local cid = host.regex.find(href, [[/manga/[^/]+/([^/]+)/?$]]) or host.regex.find(href, [[/([^/]+)/?$]])
            if cid and not seen[cid] then
                seen[cid] = true
                -- Released dates normalize through host.text; "" means the site
                -- gave nothing usable, which leaves released_at out entirely.
                local iso = host.text.date_to_iso(date)
                local ch = {
                    id = slug .. ":" .. cid,
                    chapter_num = host.text.chapter_num(label),
                    title = label,
                    url = href
                }
                if iso ~= "" then ch.released_at = iso end
                chapters[#chapters + 1] = ch
            end
        end
    end

    -- Fallback: extract chapter links from <a href> containing /chapter-
    if #chapters == 0 then
        for href, label in host.regex.gmatch(body, [[<a[^>]*href="([^"]*chapter-[^"]+)"[^>]*>\s*([^<]+)\s*</a>]]) do
            local cid = host.regex.find(href, [[/([^/]+)/?$]])
            if cid and not seen[cid] then
                seen[cid] = true
                chapters[#chapters + 1] = {
                    id = slug .. ":" .. cid,
                    chapter_num = host.text.chapter_num(label),
                    title = trim(label),
                    url = href
                }
            end
        end
    end

    log.debug("mangaread chapters slug=" .. slug .. " count=" .. tostring(#chapters))
    return host.json.encode(chapters)
end

-- ─── ABI: get_page_list ─────────────────────────────────────────────────────

function get_page_list(arg)
    local path = host.json.decode(arg)
    path = path:gsub(":", "/")
    local body = host.http.get_body(BASE .. "/manga/" .. path .. "/")
    if not body then return host.json.encode({}) end

    local pages = {}
    local pos = 1
    while true do
        local s = host.regex.find_index(body, "<img", pos)
        if not s then break end
        local e = host.regex.find_index(body, ">", s)
        if not e then break end
        local tag = body:sub(s, e)
        if host.regex.match(tag, "wp-manga-chapter-img") then
            -- data-src may have tabs/newlines between the attribute and URL value
            local src = host.regex.find(tag, [[data-src="[\t\n\s]*(https?://[^"]+)"]])
                or host.regex.find(tag, [[data-src="(https?://[^"]+)"]])
                or host.regex.find(tag, [[src="[\t\n\s]*(https?://[^"]+)"]])
                or host.regex.find(tag, [[src="(https?://[^"]+)"]])
            if src then pages[#pages + 1] = { index = #pages, url = trim(src) } end
        end
        pos = e + 1
    end

    log.debug("mangaread pages " .. path .. " count=" .. tostring(#pages))
    return host.json.encode(pages)
end


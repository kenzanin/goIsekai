-- LHTranslation plugin for goIsekai (Lua / Lunar VM)
-- Site: https://lhtranslation.net (WordPress Madara theme)
-- ABI contract version: 1

PLUGIN = {
    contract_version = 1,
    name = "LHTranslation",
    site_url = "https://lhtranslation.net",
    logo = "data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 32 32'%3E%3Ctext y='28' font-size='28'%3E🌐%3C/text%3E%3C/svg%3E",
    verify_url = "https://lhtranslation.net",
    needs_human_verify = false,
    thumb_ratio = 0.703,
    search_page_size = 24,
}

BASE = "https://lhtranslation.net"

-- ─── helpers ────────────────────────────────────────────────────────────────

local trim = host.text.trim
local unescape = host.text.unescape

-- The wrappers keep this site's 200-check and collapse the response to a body;
-- the request itself is shaped by the host, which also logs failures.
local function http_get(url)
    local resp = host.http.get(url)
    if not resp or resp.status ~= 200 then return nil end
    return resp.body
end

local function http_post(url)
    local resp = host.http.post(url, "", { ["X-Requested-With"] = "xmlhttprequest" })
    if not resp or resp.status ~= 200 then return nil end
    return resp.body
end

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
    local body = http_get(url)
    if not body then return host.json.encode({}) end

    local results = {}
    -- Parse per item: anchor <a href=".../manga/SLUG/" title="..."> followed
    -- within the same block by <img data-src="COVER">. Per-block parsing keeps
    -- covers aligned with slugs even when other /manga/ anchors intervene.
    local pos = 1
    while true do
        local s = body:find('<a%s+href="', pos)
        if not s then break end
        local e = body:find("</a>", s, true)
        if not e then break end
        local block = body:sub(s, e)
        local href, title = block:match('^<a%s+href="([^"]*/manga/[^"]+)"%s+title="([^"]*)"')
        if href then
            local slug = href:match("/manga/([^/]+)/$")
            if slug then
                local cover = block:match('data%-src="([^"]+)"') or block:match('%ssrc="([^"]+)"') or ""
                if cover:find("dflazy") then cover = "" end
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
    local body = http_get(BASE .. "/manga/" .. slug .. "/")
    -- empty table encodes as []; detail must stay an OBJECT — emit {id} only
    if not body then return host.json.encode({id = slug}) end

    local detail = { id = slug, title = "", author = "", description = "",
        cover_url = "", genres = {}, status = "" }

    -- cover: <div class="summary_image"> <a href> <img data-src="...">
    local cover_block = body:match('class="summary_image".-</a>') or ""
    detail.cover_url = trim(cover_block:match('data%-src="([^"]+)"') or cover_block:match('%ssrc="([^"]+)"') or "")

    -- title: first <h1..> inside post-title h1 variant, else <title>
    local tblock = body:match('class="post%-title[^"]*">(.-)</div>') or ""
    detail.title = unescape(trim(tblock:match("<h[1-6][^>]*>(.-)</h[1-6]>") or ""))

    -- author + artist
    local ablock = body:match('class="author%-content">(.-)</div>') or ""
    detail.author = host.text.strip_html(ablock):gsub(",", ", ")

    -- genres
    local gblock = body:match('class="genres%-content">(.-)</div>') or ""
    for g in gblock:gmatch("<a[^>]*>([^<]+)</a>") do
        local name = unescape(trim(g))
        if name ~= "" then detail.genres[#detail.genres + 1] = name end
    end

    -- status: post-status block, second summary-content (after Release)
    local sblock = body:match('class="post%-status">(.-)$') or ""
    if sblock ~= "" then
        -- find the "Status" heading then the next summary-content
        local after = sblock:match("Status%s*</h5>.-class=\"summary%-content\">%s*([^<]+)")
        if after then
            detail.status = host.text.normalize_status(trim(after))
        end
    end

    -- description: summary__content block, strip tags (may contain nested divs —
    -- capture to the "show-more" span boundary)
    local dblock = body:match('class="summary__content[^"]*">(.-)<span%s+class="[^"]*content%-readmore"') or ""
    if dblock == "" then dblock = body:match('class="summary__content[^"]*">(.-)</div>') or "" end
    detail.description = host.text.strip_html(dblock):gsub("%s+", " ")

    return host.json.encode(detail)
end

-- ─── ABI: get_chapter_list ──────────────────────────────────────────────────
-- arg: '"slug"'. Madara loads chapters via POST {base}/manga/SLUG/ajax/chapters/
-- Returns newest-first: /ajax/chapters/ returns newest first already.

function get_chapter_list(arg)
    local slug = host.json.decode(arg)
    local body = http_post(BASE .. "/manga/" .. slug .. "/ajax/chapters/")
    if not body then return host.json.encode({}) end

    local chapters = {}
    for li in body:gmatch('<li%s+class="wp%-manga%-chapter[^>]*>(.-)</li>') do
        local href = li:match('href="([^"]+)"')
        local label = trim(li:match(">([^<]*[Cc]hapter[^<]*)<") or "")
        local date = trim(li:match('chapter%-release%-date[^>]*>%s*<i[^>]*>([^<]+)</i>') or li:match('chapter%-release%-date[^>]*>%s*<a[^>]*>([^<]+)</a>') or "")
        if href and label ~= "" then
            local num = label:match("[Cc]hapter%s+([%d%.%-]+)")
            local cid = href:match("/manga/[^/]+/([^/]+)/$")
            -- released_at: Madara emits "July 7, 2026" → convert to ISO 8601
            local months = { January="01", February="02", March="03", April="04",
                May="05", June="06", July="07", August="08", September="09",
                October="10", November="11", December="12" }
            local mon, day, yr = date:match("(%a+)%s+(%d+),%s+(%d+)")
            local iso = nil
            if mon and months[mon] then
                iso = string.format("%s-%02d-%02dT12:00:00Z", yr, tonumber(months[mon]), tonumber(day))
            end
            local ch = {
                id = slug .. ":" .. (cid or label),
                chapter_num = tonumber(num) or 0,
                title = label,
                url = href
            }
            if iso then ch.released_at = iso end
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
    local body = http_get(BASE .. "/manga/" .. path .. "/")
    if not body then return host.json.encode({}) end

    local pages = {}
    -- Find all <img ...> tags carrying wp-manga-chapter-img, pull data-src
    local pos = 1
    while true do
        local s = body:find("<img", pos, true)
        if not s then break end
        local e = body:find(">", s, true)
        if not e then break end
        local tag = body:sub(s, e)
        if tag:find("wp%-manga%-chapter%-img") then
            local src = tag:match('data%-src="([^"]+)"') or tag:match('%ssrc="([^"]+)"')
            if src then pages[#pages + 1] = { url = trim(src) } end
        end
        pos = e + 1
    end

    log.debug("lhtranslation pages " .. path .. " count=" .. tostring(#pages))
    return host.json.encode(pages)
end


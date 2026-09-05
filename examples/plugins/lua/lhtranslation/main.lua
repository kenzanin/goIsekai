-- LHTranslation plugin for goIsekai (Lua / Lunar VM)
-- Site: https://lhtranslation.net (WordPress Madara theme)
-- ABI contract version: 1

PLUGIN = {
    contract_version = 1,
    name = "LHTranslation",
    verify_url = "https://lhtranslation.net",
    needs_human_verify = false,
    thumb_ratio = 0.703,
    search_page_size = 24
}

local BASE = "https://lhtranslation.net"

-- ─── helpers ────────────────────────────────────────────────────────────────

local function trim(s)
    return (s:gsub("^%s+", ""):gsub("%s+$", ""))
end

local function http_get(url)
    local req = { url = url, method = "GET", headers = {} }
    local resp = http_request(req)
    if not resp or resp.status ~= 200 then
        log.error("http status " .. (resp and resp.status or "nil") .. " for " .. url)
        return nil
    end
    return resp.body
end

local function http_post(url)
    local req = { url = url, method = "POST", headers = { ["X-Requested-With"] = "xmlhttprequest" } }
    local resp = http_request(req)
    if not resp or resp.status ~= 200 then
        log.error("http status " .. (resp and resp.status or "nil") .. " for " .. url)
        return nil
    end
    return resp.body
end

local function url_encode(s)
    return (s:gsub("([^%w%-%.%_%~])", function(c)
        return string.format("%%%02X", string.byte(c))
    end))
end

-- decode HTML entities found in titles
local function unescape(s)
    if not s then return s end
    local map = { quot = '"', amp = "&", lt = "<", gt = ">", apos = "'", nbsp = " ",
        ["#039"] = "'", ["#8217"] = "’", ["#8211"] = "–", ["#8230"] = "…" }
    return (s:gsub("&(%w+);", map):gsub("&#(%d+);", function(n)
        n = tonumber(n)
        -- ASCII range only; ponytail: higher codepoints rare in titles
        if n >= 32 and n <= 126 then return string.char(n) end
        return ""
    end))
end

-- ─── ABI: search_manga ──────────────────────────────────────────────────────
-- arg: {"query":"...","page":1}. Madara search: /?s=Q&post_type=wp-manga
-- Single page per call (Madara caps results page); return-all from this page.

function search_manga(arg)
    local args = json.decode(arg)
    local query = args.query or ""
    local page = args.page or 1

    local url = BASE .. "/?s=" .. url_encode(query) .. "&post_type=wp-manga"
    if page > 1 then
        url = url .. "&paged=" .. tostring(page)
    end
    local body = http_get(url)
    if not body then return json.encode({}) end

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
    return json.encode(results)
end

-- ─── ABI: get_manga_detail ──────────────────────────────────────────────────
-- arg: '"slug"'. Detail page fields: summary_image cover, post-title h3/h1,
-- author-content, artist-content, genres-content, post-status, summary__content.

function get_manga_detail(arg)
    local slug = json.decode(arg)
    local body = http_get(BASE .. "/manga/" .. slug .. "/")
    -- empty table encodes as []; detail must stay an OBJECT — emit {id} only
    if not body then return json.encode({id = slug}) end

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
    detail.author = unescape(trim(ablock:gsub("<[^>]+>", " "):gsub(",", ", ")))

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
            local raw = trim(after)
            local smap = {
                ["ongoing"] = "Ongoing", ["on going"] = "Ongoing", ["on-going"] = "Ongoing",
                ["completed"] = "Completed", ["complete"] = "Completed",
                ["onhold"] = "Hiatus", ["on hold"] = "Hiatus", ["hiatus"] = "Hiatus",
                ["cancelled"] = "Dropped", ["dropped"] = "Dropped",
                ["upcoming"] = "Upcoming"
            }
            detail.status = smap[raw:lower()] or raw
        end
    end

    -- description: summary__content block, strip tags (may contain nested divs —
    -- capture to the "show-more" span boundary)
    local dblock = body:match('class="summary__content[^"]*">(.-)<span%s+class="[^"]*content%-readmore"') or ""
    if dblock == "" then dblock = body:match('class="summary__content[^"]*">(.-)</div>') or "" end
    detail.description = unescape(trim(dblock:gsub("<[^>]+>", " "):gsub("%s+", " ")))

    return json.encode(detail)
end

-- ─── ABI: get_chapter_list ──────────────────────────────────────────────────
-- arg: '"slug"'. Madara loads chapters via POST {base}/manga/SLUG/ajax/chapters/
-- Returns newest-first: /ajax/chapters/ returns newest first already.

function get_chapter_list(arg)
    local slug = json.decode(arg)
    local body = http_post(BASE .. "/manga/" .. slug .. "/ajax/chapters/")
    if not body then return json.encode({}) end

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
                id = cid or label,
                chapter_num = tonumber(num) or 0,
                title = label,
                url = href
            }
            if iso then ch.released_at = iso end
            chapters[#chapters + 1] = ch
        end
    end

    log.debug("lhtranslation chapters slug=" .. slug .. " count=" .. tostring(#chapters))
    return json.encode(chapters)
end

-- ─── ABI: get_page_list ─────────────────────────────────────────────────────
-- arg: '"slug/chapter-N"'. Chapter page: <div class="reading-content"> with
-- <img data-src="..."> inside .page-break divs.

function get_page_list(arg)
    local path = json.decode(arg) -- "slug:chapter-N"
    path = path:gsub(":", "/")
    local body = http_get(BASE .. "/manga/" .. path .. "/")
    if not body then return json.encode({}) end

    local pages = {}
    for src in body:gmatch('class="wp%-manga%-chapter%-img[^>]*"%s*>') do
        -- attribute comes BEFORE class in this theme: capture from preceding tag text instead
    end
    -- Simpler: find all <img ...> tags carrying wp-manga-chapter-img, pull data-src
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
    return json.encode(pages)
end

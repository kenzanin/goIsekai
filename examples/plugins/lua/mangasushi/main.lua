-- Mangasushi plugin for goIsekai (Lua / Lunar VM)
-- Site: https://mangasushi.org (WordPress Madara theme)
-- ABI contract version: 1

PLUGIN = {
    contract_version = 1,
    name = "Mangasushi",
    site_url = "https://mangasushi.org",
    logo = "data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 32 32'%3E%3Ctext y='28' font-size='28'%3E🍣%3C/text%3E%3C/svg%3E",
    verify_url = "https://mangasushi.org",
    needs_human_verify = false,
    thumb_ratio = 0.703,
    search_page_size = 24,
    alt_title_servers = {{id = "mangadex", name = "MangaDex"}}
}

local BASE = "https://mangasushi.org"

-- ─── helpers ────────────────────────────────────────────────────────────────

local function trim(s)
    return (s:gsub("^%s+", ""):gsub("%s+$", ""))
end

local function url_encode(s)
    return (s:gsub("([^%w%-%.%_%~])", function(c)
        return string.format("%%%02X", string.byte(c))
    end))
end

local function unescape(s)
    if not s then return s end
    local map = { quot = '"', amp = "&", lt = "<", gt = ">", apos = "'", nbsp = " ",
        ["#039"] = "'", ["#8217"] = "'", ["#8211"] = "–", ["#8230"] = "…" }
    return (s:gsub("&(%w+);", map):gsub("&#(%d+);", function(n)
        n = tonumber(n)
        if n >= 32 and n <= 126 then return string.char(n) end
        return ""
    end))
end

local function http_post(url, body)
    local headers = {
        ["X-Requested-With"] = "xmlhttprequest",
        ["Content-Type"] = "application/x-www-form-urlencoded",
    }
    local req = { url = url, method = "POST", headers = headers, body = body or "" }
    local resp = http_request(req)
    if not resp or resp.status ~= 200 then
        log.error("http status " .. (resp and resp.status or "nil") .. " for " .. url)
        return nil
    end
    return resp.body
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

-- ─── ABI: search_manga ──────────────────────────────────────────────────────

function search_manga(arg)
    local args = json.decode(arg)
    local query = args.query or ""
    local page = args.page or 1

    -- Madara AJAX: POST admin-ajax.php with madara_load_more
    -- Per Madara dev docs: vars[search][keyword] for search term
    local p = tostring(page - 1)
    local form = "action=madara_load_more"
        .. "&vars%5Bpaged%5D=1"
        .. "&vars%5Btemplate%5D=madara-core%2Fcontent%2Fcontent-archive"
        .. "&vars%5Bposts_per_page%5D=25"
        .. "&vars%5Bpost_type%5D=wp-manga"
        .. "&vars%5Bpost_status%5D=publish"
        .. "&vars%5Bmanga_archives_item_layout%5D=big_thumbnail"
        .. "&vars%5Bmeta_key%5D=_wp_manga_chapter"
        .. "&vars%5Borderby%5D=meta_value_num"
        .. "&vars%5Border%5D=DESC"
        .. "&vars%5Bpage%5D=" .. p
        .. "&vars%5Bvars%5D%5Bpaged%5D=1"
        .. "&vars%5Bvars%5D%5Bpost_type%5D=wp-manga"
        .. "&vars%5Bvars%5D%5Bpost_status%5D=publish"
        .. "&vars%5Bvars%5D%5Bposts_per_page%5D=25"
        .. "&vars%5Bvars%5D%5Bmeta_key%5D=_wp_manga_chapter"
        .. "&vars%5Bvars%5D%5Borderby%5D=meta_value_num"
        .. "&vars%5Bvars%5D%5Border%5D=DESC"
        .. "&vars%5Bvars%5D%5Bmanga_archives_item_layout%5D=big_thumbnail"
        .. "&vars%5Bvars%5D%5Bpage%5D=1"
        .. "&vars%5Bvars%5D%5Bmeta_query%5D%5B0%5D%5Bkey%5D=_wp_manga_chapter_type"
        .. "&vars%5Bvars%5D%5Bmeta_query%5D%5B0%5D%5Bvalue%5D=manga"
        .. "&vars%5Bvars%5D%5Bmeta_query%5D%5B0%5D%5Bcompare%5D=="
        .. "&vars%5Bvars%5D%5Bmeta_query%5D%5Brelation%5D=AND"
    if query ~= "" then
        form = form .. "&vars%5Bvars%5D%5Bsearch%5D%5Bkeyword%5D=" .. url_encode(query)
        form = form .. "&vars%5Bsearch%5D%5Bkeyword%5D=" .. url_encode(query)
    end

    local body = http_post(BASE .. "/wp-admin/admin-ajax.php", form)
    if not body or body == "" then
        log.error("mangasushi search empty response")
        return json.encode({})
    end

    local results = {}
    local seen = {}

    -- Madara listing: <a href="URL" title="TITLE"><img data-src="COVER"></a>
    for href, title, cover in body:gmatch('<a[^>]*href="([^"]+)"[^>]*title="([^"]+)"[^>]*>.-<img[^>]*(?:data%-src|src)="([^"]+)"') do
        local slug = href:match('/manga/([^/]+)/') or href:match('/manga/([^/]+)$')
        if slug and not seen[slug] then
            seen[slug] = true
            results[#results + 1] = {
                id = slug,
                title = unescape(title),
                cover_url = trim(cover)
            }
        end
    end

    -- Fallback: page-item-detail blocks
    if #results == 0 then
        for block in body:gmatch('<div class="page%-item%-detail.-%z>') do -- dummy pattern to force scan
        end
    end

    log.debug("mangasushi search q=" .. query .. " page=" .. tostring(page) .. " found=" .. tostring(#results))
    return json.encode(results)
end

-- ─── ABI: get_manga_detail ──────────────────────────────────────────────────

function get_manga_detail(arg)
    local slug = json.decode(arg)
    local body = http_get(BASE .. "/manga/" .. slug .. "/")
    if not body then return json.encode({id = slug}) end

    local detail = { id = slug, title = "", author = "", description = "",
        cover_url = "", genres = {}, status = "" }

    -- cover
    local cover_block = body:match('class="summary_image".-</a>') or ""
    detail.cover_url = trim(cover_block:match('data%-src="([^"]+)"') or cover_block:match('src="([^"]+)"') or "")

    -- title
    local tblock = body:match('class="post%-title[^"]*">(.-)</div>') or ""
    detail.title = unescape(trim(tblock:match("<h[1-6][^>]*>(.-)</h[1-6]>") or ""))

    -- author
    local ablock = body:match('class="author%-content">(.-)</div>') or ""
    detail.author = unescape(trim(ablock:gsub("<[^>]+>", " "):gsub(",", ", ")))

    -- genres
    local gblock = body:match('class="genres%-content">(.-)</div>') or ""
    for g in gblock:gmatch("<a[^>]*>([^<]+)</a>") do
        local name = unescape(trim(g))
        if name ~= "" then detail.genres[#detail.genres + 1] = name end
    end

    -- status
    local sblock = body:match('class="post%-status">(.-)$') or ""
    if sblock ~= "" then
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

    -- description
    local dblock = body:match('class="summary__content[^"]*">(.-)<span%s+class="[^"]*content%-readmore"') or ""
    if dblock == "" then dblock = body:match('class="summary__content[^"]*">(.-)</div>') or "" end
    detail.description = unescape(trim(dblock:gsub("<[^>]+>", " "):gsub("%s+", " ")))

    return json.encode(detail)
end

-- ─── ABI: get_chapter_list ──────────────────────────────────────────────────
-- Madara chapters via POST {base}/manga/SLUG/ajax/chapters/

function get_chapter_list(arg)
    local slug = json.decode(arg)
    local body = http_post(BASE .. "/manga/" .. slug .. "/ajax/chapters/", "")
    if not body then return json.encode({}) end

    local chapters = {}
    for li in body:gmatch('<li%s+class="wp%-manga%-chapter[^>]*>(.-)</li>') do
        local href = li:match('href="([^"]+)"')
        local label = trim(li:match(">([^<]*[Cc]hapter[^<]*)<") or "")
        if href and label ~= "" then
            local num = label:match("[Cc]hapter%s+([%d%.%-]+)")
            local cid = href:match("/manga/[^/]+/([^/]+)/$")
            chapters[#chapters + 1] = {
                id = cid or label,
                chapter_num = tonumber(num) or 0,
                title = label,
                url = href
            }
        end
    end

    log.debug("mangasushi chapters slug=" .. slug .. " count=" .. tostring(#chapters))
    return json.encode(chapters)
end

-- ─── ABI: get_page_list ─────────────────────────────────────────────────────

function get_page_list(arg)
    local path = json.decode(arg)
    path = path:gsub(":", "/")
    local body = http_get(BASE .. "/manga/" .. path .. "/")
    if not body then return json.encode({}) end

    local pages = {}
    local pos = 1
    while true do
        local s = body:find("<img", pos, true)
        if not s then break end
        local e = body:find(">", s, true)
        if not e then break end
        local tag = body:sub(s, e)
        if tag:find("wp%-manga%-chapter%-img") then
            local src = tag:match('data%-src="([^"]+)"') or tag:match('src="([^"]+)"')
            if src then pages[#pages + 1] = { url = trim(src) } end
        end
        pos = e + 1
    end

    log.debug("mangasushi pages " .. path .. " count=" .. tostring(#pages))
    return json.encode(pages)
end

function getAltTitles(arg)
    local input = json.decode(arg)
    local title = input.title or ""
    local url = "https://api.mangadex.org/manga?title=" .. url_encode(title) .. "&limit=5&includes[]=manga"
    local req = {url = url, method = "GET", headers = {}}
    local resp = http_request(req)
    if not resp or resp.status ~= 200 then
        return json.encode({source = "MangaDex", titles = {}})
    end
    local ok, body = pcall(json.decode, resp.body)
    if not ok or not body or not body.data or #body.data == 0 then
        return json.encode({source = "MangaDex", titles = {}})
    end
    local attrs = body.data[1].attributes
    local out = {}
    local seen = {}
    local keep_langs = {en=true, ja=true, ["ja-ro"]=true, ko=true, ["ko-ro"]=true}
    if attrs.altTitles then
        for _, alt in ipairs(attrs.altTitles) do
            for lang, val in pairs(alt) do
                if keep_langs[lang] and val and not seen[val] then
                    seen[val] = true
                    out[#out + 1] = val
                end
            end
        end
    end
    return json.encode({source = "MangaDex", titles = out})
end

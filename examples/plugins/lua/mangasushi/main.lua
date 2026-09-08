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
    alt_title_servers = {
        {id = "mangadex",     name = "MangaDex",     kind = "titles"},
        {id = "mangaupdates", name = "MangaUpdates", kind = "both"}
    },
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

    -- Madara GET search: admin-ajax.php returns 0 bytes, use server-rendered GET instead
    local url = BASE .. "/?s=" .. url_encode(query) .. "&post_type=wp-manga"
    local body = http_get(url)
    if not body or body == "" then
        log.error("mangasushi search empty response")
        return json.encode({})
    end

    local results = {}
    local seen = {}

    -- Parse .post-title > h3 > a for title + href
    -- Structure: <div class="post-title"><h3 class="h4"><a href="URL">TITLE</a></h3></div>
    for href, title in body:gmatch('post%-title[^>]*>.-<a[^>]*href="([^"]+)"[^>]*>([^<]+)</a>') do
        local slug = href:match('/manga/([^/]+)/') or href:match('/manga/([^/]+)$')
        if slug and not seen[slug] then
            seen[slug] = true
            -- Find cover: search nearby for data-src or src with image URL
            local cover = ""
            local pos = body:find(slug, 1, true)
            if pos then
                local area = body:sub(math.max(1, pos - 3000), math.min(#body, pos + 3000))
                -- Madara lazy images: data-src with possible tabs/newlines before URL
                cover = area:match('data%-src="[\t\n%s]*(https?://[^"]+)"')
                    or area:match('data%-src="(https?://[^"]+)"')
                    or area:match('src="(https?://[^"]+)"')
                if cover then cover = trim(cover) end
            end
            results[#results + 1] = {
                id = slug,
                title = unescape(title),
                cover_url = cover or ""
            }
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
    local seen = {}

    -- Madara chapter list: <li class="wp-manga-chapter"><a href="URL">LABEL</a>...<i>DATE</i></li>
    for li in body:gmatch('<li[^>]*class="wp%-manga%-chapter[^"]*"[^>]*>(.-)</li>') do
        local href = li:match('href="([^"]+)"')
        local label = trim(li:match('>([^<]*[Cc]hapter[^<]*)<') or "")
        local date = trim(li:match('chapter%-release%-date[^>]*>.-<i>([^<]+)</i>') or "")
        if href and label ~= "" then
            local num = label:match('[Cc]hapter%s+([%d%.%-]+)')
            local cid = href:match('/manga/[^/]+/([^/]+)/?$') or href:match('/([^/]+)/?$')
            if cid and not seen[cid] then
                seen[cid] = true
                chapters[#chapters + 1] = {
                    id = cid,
                    chapter_num = tonumber(num) or 0,
                    title = label,
                    uploaded_at = date,
                    url = href
                }
            end
        end
    end

    -- Fallback: extract chapter links from <a href> containing /chapter-
    if #chapters == 0 then
        for href, label in body:gmatch('<a[^>]*href="([^"]*chapter-[^"]+)"[^>]*>%s*([^<]+)%s*</a>') do
            local cid = href:match('/([^/]+)/?$')
            if cid and not seen[cid] then
                seen[cid] = true
                local num = label:match('[Cc]hapter%s+([%d%.%-]+)')
                chapters[#chapters + 1] = {
                    id = cid,
                    chapter_num = tonumber(num) or 0,
                    title = trim(label),
                    url = href
                }
            end
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
            -- data-src may have tabs/newlines between the attribute and URL value
            local src = tag:match('data%-src="[\t\n%s]*(https?://[^"]+)"')
                or tag:match('data%-src="(https?://[^"]+)"')
                or tag:match('src="[\t\n%s]*(https?://[^"]+)"')
                or tag:match('src="(https?://[^"]+)"')
            if src then pages[#pages + 1] = { index = #pages, url = trim(src) } end
        end
        pos = e + 1
    end

    log.debug("mangasushi pages " .. path .. " count=" .. tostring(#pages))
    return json.encode(pages)
end


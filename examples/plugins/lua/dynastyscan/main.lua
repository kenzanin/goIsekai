-- DynastyScan plugin for goIsekai (Lua / Lunar VM)
-- Site: https://dynasty-scans.com (Dynasty Reader, Ruby on Rails)
-- ABI contract version: 1

PLUGIN = {
    contract_version = 1,
    name = "DynastyScan",
    site_url = "https://dynasty-scans.com",
    logo = "logo.png",
    verify_url = "https://dynasty-scans.com",
    needs_human_verify = false,
    thumb_ratio = 0.703,
}

BASE = "https://dynasty-scans.com"

local trim = host.text.trim

-- The site answers most things with JSON appended to the normal path. Search is
-- the exception: there is no /search.json, so it is the one page scraped here.
local function get_json(url)
    local body = host.http.get_body(url, { ["accept"] = "application/json" })
    if not body or body == "" then return nil end
    local ok, decoded = pcall(host.json.decode, body)
    if not ok or decoded == nil then return nil end
    return decoded
end

local function absolute(path)
    if path == nil or path == "" then return "" end
    if path:match("^https?://") then return path end
    return BASE .. path
end

-- ─── ABI: search_manga ──────────────────────────────────────────────────────

function search_manga(arg)
    local args = host.json.decode(arg)
    local query = args.query or ""
    local page = args.page or 1
    local genres = args.genres or {}

    local url
    if #genres > 0 then
        -- Tag archives: /tags/<slug>/, with ?page=N rather than a path segment.
        url = BASE .. "/tags/" .. host.text.url_encode(genres[1]) .. "/"
        if page > 1 then url = url .. "?page=" .. tostring(page) end
    else
        url = BASE .. "/search?q=" .. host.text.url_encode(query)
        if page > 1 then url = url .. "&page=" .. tostring(page) end
    end

    local body = host.http.get_body(url)
    local results = {}
    if not body or body == "" then
        log.debug("dynastyscan search q=" .. query .. " empty body")
        return host.json.encode(results)
    end

    -- Results are definition lists: <dd>...<a href="/series/slug" class="name">TITLE</a>
    -- by <a href="/authors/x">AUTHOR</a>...</dd>. Matching the whole entry keeps
    -- the title paired with the right slug when titles repeat across the page.
    local seen = {}
    for block in host.regex.gmatch(body, [[(?s)<dd>(.*?)</dd>]]) do
        local slug = host.regex.find(block, [[href="/series/([a-z0-9_]+)"]])
        local title = host.regex.find(block, [[class="name"[^>]*>([^<]+)</a>]])
        if slug and title and not seen[slug] then
            seen[slug] = true
            local cover = host.regex.find(block, [[<img[^>]*src="([^"]+)"]])
            results[#results + 1] = {
                id = slug,
                title = host.text.unescape(trim(title)),
                cover_url = cover and absolute(cover) or "",
            }
        end
    end

    log.debug("dynastyscan search q=" .. query .. " page=" .. tostring(page) .. " found=" .. tostring(#results))
    return host.json.encode(results)
end

-- ─── ABI: get_manga_detail ──────────────────────────────────────────────────
-- /series/<slug>.json also carries the chapter list in "taggings", so the detail
-- and the chapter list cost one request between them.

function get_manga_detail(arg)
    local slug = host.json.decode(arg)
    local data = get_json(BASE .. "/series/" .. host.text.url_encode(slug) .. ".json")
    -- Never {}: the host reads an object-returning ABI as a detail, and a nil id
    -- makes an unreachable source look like an empty series.
    if type(data) ~= "table" then return host.json.encode({ id = slug }) end

    local detail = {
        id = slug,
        title = data.name or "",
        author = "",
        description = "",
        cover_url = absolute(data.cover),
        genres = {},
        status = "",
    }

    detail.description = host.regex.replace(host.text.strip_html(data.description or ""), [[\s+]], " ")

    -- Tag types are Author / General (the genre set) / Status. "General" is the
    -- one that carries genres here, and Status carries the reading status - the
    -- top-level "type" field is the container kind (Series, Anthology, Doujin),
    -- which is not a status and reads as nonsense in the detail view.
    for _, tag in ipairs(data.tags or {}) do
        if tag.type == "Author" then
            detail.author = detail.author == "" and tag.name or (detail.author .. ", " .. tag.name)
        elseif tag.type == "General" then
            detail.genres[#detail.genres + 1] = tag.name
        elseif tag.type == "Status" and detail.status == "" then
            detail.status = host.text.normalize_status(tag.name or "")
        end
    end

    -- Alternate titles are what make this source match in a cross-source search.
    local aliases = data.aliases
    if type(aliases) == "string" and aliases ~= "" then
        detail.alt_title = aliases
    elseif type(aliases) == "table" then
        for _, a in ipairs(aliases) do
            if type(a) == "string" and a ~= "" then detail.alt_title = a break end
        end
    end

    return host.json.encode(detail)
end

-- ─── ABI: get_chapter_list ──────────────────────────────────────────────────

function get_chapter_list(arg)
    local slug = host.json.decode(arg)
    local data = get_json(BASE .. "/series/" .. host.text.url_encode(slug) .. ".json")

    local chapters = {}
    for _, tag in ipairs((data and data.taggings) or {}) do
        local permalink = tag.permalink
        if permalink and permalink ~= "" then
            local label = trim(tag.title or "")
            chapters[#chapters + 1] = {
                id = permalink,
                chapter_num = host.text.chapter_num(label),
                title = label,
                url = BASE .. "/chapters/" .. permalink,
            }
        end
    end

    -- "taggings" is newest-first already (it is the site's own release feed
    -- order), but the ABI requires it explicitly and a re-sort is cheap.
    table.sort(chapters, function(a, b)
        return (a.chapter_num or 0) > (b.chapter_num or 0)
    end)

    log.debug("dynastyscan chapters slug=" .. slug .. " count=" .. tostring(#chapters))
    return host.json.encode(chapters)
end

-- ─── ABI: get_page_list ─────────────────────────────────────────────────────

function get_page_list(arg)
    local permalink = host.json.decode(arg)
    local data = get_json(BASE .. "/chapters/" .. host.text.url_encode(permalink) .. ".json")

    local pages = {}
    for _, p in ipairs((data and data.pages) or {}) do
        local url = absolute(p.url)
        if url ~= "" then
            pages[#pages + 1] = { index = #pages, url = url }
        end
    end

    log.debug("dynastyscan pages " .. permalink .. " count=" .. tostring(#pages))
    return host.json.encode(pages)
end

-- ─── get_genres (optional export) ──────────────────────────────────────────
-- Read from the site's own tag navigation rather than hardcoded, so the list
-- cannot drift from what the archive URLs actually accept.

function get_genres()
    local body = host.http.get_body(BASE .. "/")
    local genres = {}
    if not body or body == "" then return host.json.encode(genres) end

    local seen = {}
    for slug, name in host.regex.gmatch(body, [[href="https://dynasty%-scans%.com/tags/([a-z0-9_%-]+)/"[^>]*>([^<]+)</]]) do
        local label = host.text.unescape(trim(name))
        if slug ~= "" and label ~= "" and not seen[slug] then
            seen[slug] = true
            genres[#genres + 1] = { name = label, slug = slug }
        end
    end

    log.debug("dynastyscan genres found=" .. tostring(#genres))
    return host.json.encode(genres)
end
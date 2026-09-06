-- ValirScans plugin for goIsekai
-- Site: https://valirscans.org
-- ABI contract version: 1 (matches pkg/types ContractVersion)
--
-- Site profile: custom Next.js reader (not WordPress/Madara). Data sources:
--   search:  GET /api/search?q=QUERY        -> JSON {series:[...]}
--   detail:  GET /series/comic/{urlSlug}    -> HTML; JSON-LD Book block
--            carries name/author/image/description/genre[]; the series
--            status lives in the flight payload next to coverImage.
--   chapter: /series/comic/{urlSlug}/chapter/{N} -> HTML; page images are
--            direct https://media.valirscans.org/.../p-*.webp URLs (200
--            with or without Referer).

PLUGIN = {
    contract_version = 1,
    name = "ValirScans",
    site_url = "https://valirscans.org",
    logo = "https://valirscans.org/favicon.png",
    verify_url = "https://valirscans.org",
    needs_human_verify = false,
    thumb_ratio = 0.703,
    search_page_size = 24,
    alt_title_servers = {{id = "mangadex", name = "MangaDex"}}
}

BASE = "https://valirscans.org"
MEDIA = "https://media.valirscans.org"

-- normalizeStatus maps a raw status string to a canonical host value.
-- Canonical set: Ongoing, Completed, Hiatus, Dropped, Upcoming.
-- Unknown values pass through as-is.
local function normalizeStatus(s)
    local raw = (s or ""):lower()
    if raw:find("ongo") or raw:find("releas") or raw:find("publish") then return "Ongoing" end
    if raw:find("complet") or raw:find("finish") then return "Completed" end
    if raw:find("hiatus") or raw:find("on.?hold") or raw:find("onhold") then return "Hiatus" end
    if raw:find("drop") or raw:find("cancel") then return "Dropped" end
    if raw:find("upcom") or raw:find("not.?publish") then return "Upcoming" end
    return s or ""
end

-- ─── helpers ───────────────────────────────────────────────────────────────

function url_encode(s)
    return s:gsub("([^%w%-%.%_%~])", function(c)
        return string.format("%%%02X", string.byte(c))
    end)
end

-- Escape Lua pattern magic chars (slugs are [a-z0-9-], but '-' is the
-- lazy quantifier — raw slugs in gmatch/find patterns silently fail).
function lua_escape(s)
    return (s:gsub("[%-%.%+%[%]%(%)%$%^%%%?%*]", "%%%0"))
end

function http_get(url, extra_headers)
    local req = { url = url, method = "GET", headers = {} }
    if extra_headers then
        for k, v in pairs(extra_headers) do
            req.headers[k] = v
        end
    end
    local resp = http_request(req)
    if not resp then
        log.error("http_request returned nil for " .. url)
    elseif resp.status ~= 200 then
        log.error("http status " .. tostring(resp.status) .. " for " .. url)
    end
    return resp
end

function titlecase(s)
    if s == nil then return "" end
    return s:sub(1, 1) .. s:sub(2):lower()
end

-- Extract the <script type="application/ld+json"> block whose decoded JSON
-- carries "@type":"Book". Returns decoded table or nil.
function jsonld_book(html)
    local i = 1
    while true do
        local s, e = string.find(html, '<script type="application/ld%+json">', i)
        if not s then return nil end
        local e2 = string.find(html, '</script>', e + 1)
        if not e2 then return nil end
        local chunk = html:sub(e + 1, e2 - 1)
        if string.find(chunk, '"@type"%s*:%s*"Book"', 1) then
            local ok, obj = pcall(json.decode, chunk)
            if ok and type(obj) == "table" then
                return obj
            end
        end
        i = e2 + 1
    end
end

-- Series status lives in the flight payload right after the series'
-- coverImage/bannerImage pair. Anchor on the cover path from JSON-LD and
-- read the following "status":"X" within a bounded window.
function flight_status(html, cover_path)
    if not cover_path or cover_path == "" then return "" end
    local needle = '\\"coverImage\\":\\"' .. cover_path .. '\\"'
    local i = string.find(html, needle, 1, true)
    if not i then return "" end
    local status = string.match(html:sub(i, i + 600), '\\"status\\":\\"([A-Z_]+)\\"')
    return status or ""
end

-- ─── ABI: search_manga(arg) ────────────────────────────────────────────────
-- arg: {"query":"...","page":1}  ->  array of {id, title, cover_url}
function search_manga(arg)
    local args = json.decode(arg)
    local query = args.query or ""
    log.debug("search q=" .. query)
    local resp = http_get(BASE .. "/api/search?q=" .. url_encode(query))
    if not resp or resp.status ~= 200 then
        return json.encode({})
    end
    local ok, data = pcall(json.decode, resp.body)
    if not ok or not data or not data.series then
        return json.encode({})
    end
    local out = {}
    for _, r in ipairs(data.series) do
        local cover = r.coverImage or ""
        if cover ~= "" and not string.find(cover, "^https?://") then
            cover = BASE .. cover
        end
        out[#out + 1] = {
            id = r.urlSlug or r.slug,
            title = r.title or "",
            cover_url = cover
        }
    end
    log.debug("search: found " .. #out .. " results for q=" .. query)
    return json.encode(out)
end

-- ─── ABI: get_manga_detail(arg) ────────────────────────────────────────────
-- arg: '"urlSlug"'  ->  {id, title, author, description, cover_url, genres, status}
function get_manga_detail(arg)
    local manga_id = json.decode(arg) -- plain string (urlSlug)
    local resp = http_get(BASE .. "/series/comic/" .. manga_id)
    if not resp or resp.status ~= 200 then
        return json.encode({id = manga_id}) -- detail must stay an OBJECT
    end
    local html = resp.body
    local detail = { id = manga_id, title = "", author = "", description = "",
                     cover_url = "", genres = {}, status = "" }

    local book = jsonld_book(html)
    if book then
        detail.title = book.name or ""
        if type(book.author) == "table" then
            detail.author = book.author.name or ""
        elseif type(book.author) == "string" then
            detail.author = book.author
        end
        detail.description = book.description or ""
        local img = book.image or ""
        if img ~= "" then
            detail.cover_url = img
            if not string.find(img, "^https?://") then
                detail.cover_url = BASE .. img
            end
        end
        if type(book.genre) == "table" then
            local gs = {}
            for _, g in ipairs(book.genre) do
                gs[#gs + 1] = g
            end
            detail.genres = gs
        end
        -- status: anchor on the cover path (relative) in the flight payload
        local status = flight_status(html, img)
        if status ~= "" then detail.status = normalizeStatus(titlecase(status)) end
    end

    -- Fallbacks for anything JSON-LD missed (or if the block was absent)
    if detail.title == "" then
        detail.title = string.match(html, '<meta property="og:title" content="([^"]*)"') or ""
    end
    if detail.cover_url == "" then
        local c = string.match(html,
            '<meta property="og:image" content="([^"]*)"') or ""
        if c ~= "" then detail.cover_url = c end
    end
    if detail.status == "" then
        local st = flight_status(html, string.match(detail.cover_url,
            '(https?://[^/]+)?(/uploads/series/[^"]+)') or detail.cover_url)
        if st ~= "" then detail.status = normalizeStatus(titlecase(st)) end
    end

    log.debug("detail: " .. detail.title .. " | " .. detail.status)
    return json.encode(detail)
end

-- ─── ABI: get_chapter_list(arg) ────────────────────────────────────────────
-- arg: '"urlSlug"'  ->  array of {id, manga_id, chapter_num, title, url, uploaded_at}
-- Chapters are newest-first (descending number) per ABI convention.
function get_chapter_list(arg)
    local manga_id = json.decode(arg)
    local resp = http_get(BASE .. "/series/comic/" .. manga_id)
    if not resp or resp.status ~= 200 then
        return json.encode({})
    end
    local html = resp.body

    -- Chapter rows are <a href="/series/comic/{slug}/chapter/{N}">; the
    -- same URL repeats for the "latest chapter" card, so dedupe by number.
    local nums = {}
    local seen = {}
    local pat = '/series/comic/' .. lua_escape(manga_id) .. '/chapter/(%d+)'
    for n in string.gmatch(html, pat) do
        if not seen[n] then
            seen[n] = true
            nums[#nums + 1] = tonumber(n)
        end
    end
    table.sort(nums, function(a, b) return a > b end)

    local chapters = {}
    for _, n in ipairs(nums) do
        chapters[#chapters + 1] = {
            id = manga_id .. ":" .. tostring(n),
            manga_id = manga_id,
            chapter_num = n,
            title = "Chapter " .. tostring(n),
            url = BASE .. "/series/comic/" .. manga_id .. "/chapter/" .. tostring(n),
            uploaded_at = ""
        }
    end
    log.debug("chapters: " .. #chapters .. " for " .. manga_id)
    return json.encode(chapters)
end

-- ─── ABI: get_page_list(arg) ───────────────────────────────────────────────
-- arg: '"urlSlug:N"' (chapter id from get_chapter_list)  ->  array of {url}
function get_page_list(arg)
    local chapter_id = json.decode(arg) -- e.g. "urlSlug:37"
    local manga_id, num = string.match(chapter_id, "^(.-):(%d+)$")
    if not manga_id or not num then
        return json.encode({})
    end
    local resp = http_get(BASE .. "/series/comic/" .. manga_id .. "/chapter/" .. num)
    if not resp or resp.status ~= 200 then
        return json.encode({})
    end
    local html = resp.body

    -- Page images are direct media.valirscans.org URLs embedded in reading
    -- order; dedupe keeps order (chapter cover can repeat via srcset).
    local pages = {}
    local seen = {}
    local pat = '(https://media%.valirscans%.org/series/' .. lua_escape(manga_id) .. '/%d+/p%-[^"\\]+%.webp)'
    for u in string.gmatch(html, pat) do
        if not seen[u] then
            seen[u] = true
            pages[#pages + 1] = { url = u }
        end
    end
    log.debug("pages: " .. #pages .. " for chapter " .. manga_id .. ":" .. num)
    return json.encode(pages)
end

-- ─── ABI: getAltTitles(arg) ────────────────────────────────────────────────
-- Each plugin carries its own alt-title source (MangaDex API) so no plugin
-- depends on another. arg: {"title":"...","server":"..."} -> {source, titles}
function getAltTitles(arg)
    local input = json.decode(arg)
    local title = input.title or ""
    local url = "https://api.mangadex.org/manga?title=" .. url_encode(title) ..
        "&limit=5&includes[]=manga"
    local resp = http_request({url = url, method = "GET", headers = {}})
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
    local keep_langs = {en = true, ja = true, ["ja-ro"] = true, ko = true, ["ko-ro"] = true}
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

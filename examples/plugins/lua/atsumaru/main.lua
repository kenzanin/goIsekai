-- Atsumaru plugin for goIsekai (Lua / Lunar VM)
-- Site: https://atsu.moe
-- ABI contract version: 1

PLUGIN = {
    contract_version = 1,
    name = "Atsumaru",
    site_url = "https://atsu.moe",
    logo = "logo.png",
    verify_url = "https://atsu.moe",
    needs_human_verify = false,
    thumb_ratio = 0.703,
}

BASE = "https://atsu.moe"
-- Images are not served from the site itself: atsu.moe/static/... is 410 Gone and
-- cdn.atsu.moe serves the same path, so every relative image path is resolved
-- against the CDN rather than the origin.
CDN = "https://cdn.atsu.moe"

local HEADERS = {
    ["accept"] = "application/json",
}

-- ─── helpers ────────────────────────────────────────────────────────────────

local trim = host.text.trim

-- Image paths come back in three shapes: "/static/posters/x.jpg" from search,
-- "posters/x.jpg" from the detail payload, and already-absolute URLs. Only the
-- relative ones need the CDN prefix.
local function image_url(path)
    if path == nil or path == "" then return "" end
    if path:match("^https?://") then return path end
    if path:sub(1, 2) == "//" then return "https:" .. path end
    path = path:gsub("^/static/", ""):gsub("^static/", "")
    return CDN .. "/static/" .. path
end

local function get_json(url)
    local body = host.http.get_body(url, HEADERS)
    if not body or body == "" then return nil end
    local ok, decoded = pcall(host.json.decode, body)
    if not ok or decoded == nil then return nil end
    return decoded
end

-- ─── ABI: search_manga ──────────────────────────────────────────────────────
-- Typesense-backed search over atsu.moe/collections. The filter hides drafts and
-- noise entries; without a query the site wants "*".

function search_manga(arg)
    local args = host.json.decode(arg)
    local query = args.query or ""
    local page = args.page or 1

    local filter = "hidden:!=true && views:>0 && medium:!=[`Novel`]"
    -- Genre browsing: the ids come from get_genres, which reads the site's own
    -- availableFilters, and the search endpoint filters on them directly.
    if args.genres and #args.genres > 0 then
        local ids = {}
        for _, g in ipairs(args.genres) do
            ids[#ids + 1] = "`" .. tostring(g) .. "`"
        end
        filter = filter .. " && genreIds:=[" .. table.concat(ids, ",") .. "]"
    end

    local url = BASE .. "/collections/manga/documents/search?q="
        .. host.text.url_encode(query ~= "" and query or "*")
        .. "&filter_by=" .. host.text.url_encode(filter)
        .. "&query_by=" .. host.text.url_encode("title,englishTitle,otherNames,authors")
        .. "&page=" .. tostring(page)
        .. "&per_page=20"

    local data = get_json(url)
    local results = {}
    if not data or type(data.hits) ~= "table" then
        log.debug("atsumaru search q=" .. query .. " page=" .. tostring(page) .. " no hits payload")
        return host.json.encode(results)
    end

    for _, hit in ipairs(data.hits) do
        local doc = hit.document
        if type(doc) == "table" and doc.id then
            local title = doc.title or doc.englishTitle or ""
            local alt = doc.englishTitle or ""
            results[#results + 1] = {
                id = doc.id,
                title = title,
                cover_url = image_url(doc.poster or doc.posterSmall),
                -- The alternative title is what makes a cross-source search match:
                -- Atsumaru carries both an original and an English title.
                alt_title = alt ~= "" and alt ~= title and alt or nil,
            }
        end
    end

    log.debug("atsumaru search q=" .. query .. " page=" .. tostring(page) .. " found=" .. tostring(#results))
    return host.json.encode(results)
end

-- ─── ABI: get_manga_detail ──────────────────────────────────────────────────

function get_manga_detail(arg)
    local id = host.json.decode(arg)
    local data = get_json(BASE .. "/api/manga/page?id=" .. host.text.url_encode(id))
    local page = data and data.mangaPage
    -- Never return {}: the ABI reads object-returning functions as a detail and
    -- a nil id makes the host treat an unreachable source as an empty series.
    if type(page) ~= "table" then return host.json.encode({ id = id }) end

    local poster = page.poster
    if type(poster) == "table" then poster = poster.image or poster.id end

    local detail = {
        id = id,
        title = page.title or "",
        author = "",
        description = host.text.strip_html(page.synopsis or ""),
        cover_url = image_url(poster),
        genres = {},
        status = host.text.normalize_status(page.status or ""),
    }

    for _, a in ipairs(page.authors or {}) do
        if a.name then
            detail.author = detail.author == "" and a.name or (detail.author .. ", " .. a.name)
        end
    end

    for _, g in ipairs(page.genres or {}) do
        if g.name then detail.genres[#detail.genres + 1] = g.name end
    end

    return host.json.encode(detail)
end

-- ─── ABI: get_chapter_list ──────────────────────────────────────────────────

function get_chapter_list(arg)
    local id = host.json.decode(arg)
    local data = get_json(BASE .. "/api/manga/allChapters?mangaId=" .. host.text.url_encode(id))
    local raw = data and data.chapters or {}

    local chapters = {}
    for _, c in ipairs(raw) do
        if c.id then
            -- get_page_list only receives the chapter id, so the manga id rides
            -- along in a composite; the page list needs both to query the API.
            local label = trim(c.title or ("Chapter " .. tostring(c.number or "")))
            chapters[#chapters + 1] = {
                id = id .. ":" .. c.id,
                chapter_num = host.text.chapter_num(label),
                title = label,
            }
        end
    end

    -- The ABI requires newest-first. The site returns ascending order, which
    -- would make "continue reading" resume from the oldest chapter.
    table.sort(chapters, function(a, b)
        return (a.chapter_num or 0) > (b.chapter_num or 0)
    end)

    log.debug("atsumaru chapters id=" .. id .. " count=" .. tostring(#chapters))
    return host.json.encode(chapters)
end

-- ─── ABI: get_page_list ─────────────────────────────────────────────────────

function get_page_list(arg)
    local composite = host.json.decode(arg)
    local mangaID, chapterID = composite:match("^([^:]+):(.+)$")
    if not mangaID or not chapterID then
        log.error("atsumaru: chapter id is not a mangaId:chapterId pair: " .. tostring(composite))
        return host.json.encode({})
    end

    local data = get_json(BASE .. "/api/read/chapter?mangaId="
        .. host.text.url_encode(mangaID)
        .. "&chapterId=" .. host.text.url_encode(chapterID))
    local read = data and data.readChapter
    local list = type(read) == "table" and read.pages or nil

    local pages = {}
    for _, p in ipairs(list or {}) do
        local url = image_url(p.image)
        if url ~= "" then
            pages[#pages + 1] = { index = #pages, url = url }
        end
    end

    log.debug("atsumaru pages " .. mangaID .. ":" .. chapterID .. " count=" .. tostring(#pages))
    return host.json.encode(pages)
end

-- ─── get_genres (optional export) ──────────────────────────────────────────
-- Taken from the site's own category slugs, which the search filter accepts as
-- `genreIds:=[...]` entries.

-- Taken from the site's own /api/explore/availableFilters, which the search
-- filter consumes as `genreIds:=[...]`. The adult entries are left out: the
-- search filter already caps content rating, and listing them here would put
-- them in front of every user of the genre picker.
local GENRES = {
    { name = "Action", slug = "39" },
    { name = "Adventure", slug = "37" },
    { name = "Comedy", slug = "6" },
    { name = "Drama", slug = "31" },
    { name = "Fantasy", slug = "36" },
    { name = "Historical", slug = "45" },
    { name = "Horror", slug = "44" },
    { name = "Martial Arts", slug = "29" },
    { name = "Mystery", slug = "32" },
    { name = "Psychological", slug = "18" },
    { name = "Romance", slug = "9" },
    { name = "Sci-Fi", slug = "1" },
    { name = "Slice of Life", slug = "7" },
    { name = "Supernatural", slug = "22" },
    { name = "Thriller", slug = "19" },
    { name = "Tragedy", slug = "5" },
}

function get_genres()
    return host.json.encode(GENRES)
end
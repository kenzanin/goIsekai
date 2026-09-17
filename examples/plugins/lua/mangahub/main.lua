-- MangaHub.io plugin for goIsekai (Lua / Lunar VM)
-- Site: https://mangahub.io — MangaHub GraphQL API at api.mghcdn.com
-- Source id m01. The API answers 404 without Origin/Referer on the POST;
-- mhub_access is only needed by the chapter (pages) resolver.
-- ABI contract version: 1 (matches pkg/types ContractVersion)

PLUGIN = {
    contract_version = 1,
    name = "MangaHub",
    site_url = "https://mangahub.io",
    logo = "logo.png",
    verify_url = "https://mangahub.io",
    needs_human_verify = false,
    thumb_ratio = 0.703,
    search_page_size = 30,
}

local GRAPHQL_URL = "https://api.mghcdn.com/graphql"
local SITE_URL = "https://mangahub.io"
local SOURCE_ID = "m01"
local IMG_CDN = "https://imgx.mghcdn.com/"
local THUMB_CDN = "https://thumb.mghcdn.com/"

-- Cached access key, dropped whenever the API refuses.
local cachedKey = nil

-- Last refusal message, so callers surface it instead of a silent empty list.
local gqlError = nil

-- ─── helpers ───────────────────────────────────────────────────────────────

-- A bare GET only hands back the key this session/IP already exhausted, so
-- every chapter comes back rate limited. Presenting a cookie value the server
-- has never issued makes it mint a genuinely new key, and ?reloadKey=1 is what
-- triggers the re-issue. Each key so obtained carries four chapters of quota,
-- and an explicit Cookie header overrides whatever the host cookie jar holds.
local function fetch_access_key()
    local url = SITE_URL .. "/chapter/martial-peak/chapter-"
        .. tostring(1000 + math.random(0, 1999)) .. "?reloadKey=1"
    local resp = host.http.get(url, {
        ["Referer"] = SITE_URL .. "/manga/martial-peak",
        ["Accept"] = "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
        ["Sec-Fetch-Dest"] = "document",
        ["Sec-Fetch-Mode"] = "navigate",
        ["Sec-Fetch-Site"] = "same-origin",
        ["Upgrade-Insecure-Requests"] = "1",
        ["Cookie"] = "mhub_access=0000000000000000000000000000cafe",
    })
    if not resp or not resp.headers then return nil end

    local cookie = resp.headers["Set-Cookie"] or ""
    local key = cookie:match("mhub_access=([^;]+)")
    if not key or key == "" then return nil end
    return key
end

-- Escape a string for embedding in a GraphQL string literal.
local function escape_gql(s)
    return (s:gsub("\\", "\\\\"):gsub('"', '\\"'))
end

-- Run a GraphQL query, dropping the cached key and retrying once when the API
-- refuses. MangaHub reports both expired keys and its "API rate limit
-- excessed" refusal as HTTP 200 with an errors array and a null payload, so the
-- status code alone cannot tell a good answer from a refused one. The message is
-- kept in gqlError so callers can surface it.
local function graphql_query(query)
    gqlError = nil

    for _ = 1, 2 do
        local key = cachedKey
        if not key then
            key = fetch_access_key()
            if key then cachedKey = key end
        end
        if not key then
            log.error("mangahub: no mhub_access key available")
            return nil
        end

        local resp = host.http.post(GRAPHQL_URL, host.json.encode({ query = query }), {
            ["Content-Type"] = "application/json",
            ["Origin"] = SITE_URL,
            ["Referer"] = SITE_URL .. "/",
            ["x-mhub-access"] = key,
        })

        if not resp or resp.status < 200 or resp.status >= 300 then
            log.info("mangahub: GraphQL status " .. tostring(resp and resp.status or 0) .. ", refreshing key")
            cachedKey = nil
        else
            local parsed = host.json.decode(resp.body)
            if not parsed then
                log.error("mangahub: unparseable GraphQL response")
                return nil
            end

            local errors = parsed.errors
            if errors and #errors > 0 then
                gqlError = errors[1].message or "GraphQL error"
            elseif not parsed.data then
                gqlError = "GraphQL response carried no data"
            else
                return parsed
            end

            log.warn("mangahub: GraphQL refused (" .. gqlError .. "), retrying with a fresh key")
            cachedKey = nil
        end
    end

    return nil
end

-- ─── ABI: search_manga ─────────────────────────────────────────────────────
-- arg: {"query":"...","page":1}
function search_manga(arg)
    local args = host.json.decode(arg) or {}
    local query = args.query or ""
    local page = args.page or 1
    local offset = (page - 1) * 30
    log.info("mangahub search: q=" .. query .. " page=" .. tostring(page))

    local gql = '{search(x: ' .. SOURCE_ID .. ', q: "' .. escape_gql(query)
        .. '", genre: "all", mod: POPULAR, offset: ' .. tostring(offset) .. ') {rows {title, slug, image}}}'

    local data = graphql_query(gql)
    local rows = {}
    if data and data.data and data.data.search then
        rows = data.data.search.rows or {}
    end

    local results = {}
    for _, row in ipairs(rows) do
        local cover = row.image or ""
        if cover ~= "" and cover:sub(1, 4) ~= "http" then
            cover = THUMB_CDN .. cover
        end
        results[#results + 1] = {
            id = row.slug,
            title = row.title or "",
            cover_url = cover,
        }
    end

    log.info("mangahub search: found " .. tostring(#results) .. " results for q=" .. query)
    return host.json.encode(results)
end

-- ─── ABI: get_manga_detail ─────────────────────────────────────────────────
-- arg: '"slug"'. A failed lookup still returns an object: an empty Lua table
-- encodes as [], which cannot decode into a detail record.
function get_manga_detail(arg)
    local slug = host.json.decode(arg)
    if not slug then return host.json.encode({ id = "" }) end

    log.info("mangahub detail: slug=" .. slug)

    local gql = '{manga(x: ' .. SOURCE_ID .. ', slug: "' .. escape_gql(slug)
        .. '") {title, slug, status, image, author, artist, genres, description, alternativeTitle}}'

    local data = graphql_query(gql)
    local m = data and data.data and data.data.manga
    if not m then return host.json.encode({ id = slug }) end

    local cover = m.image or ""
    if cover ~= "" and cover:sub(1, 4) ~= "http" then
        cover = THUMB_CDN .. cover
    end

    -- status may arrive as an enum string or as a numeric code; the host wants a
    -- string either way.
    local status = m.status
    if status == nil then status = "" else status = tostring(status) end

    -- genres arrives as one comma-separated string; the ABI wants a list.
    local genres = {}
    if type(m.genres) == "string" then
        for part in m.genres:gmatch("[^,]+") do
            local name = host.text.trim(part)
            if name ~= "" then genres[#genres + 1] = name end
        end
    elseif type(m.genres) == "table" then
        genres = m.genres
    end

    return host.json.encode({
        id = m.slug or slug,
        title = m.title or "",
        cover_url = cover,
        author = m.author or "",
        description = m.description or "",
        status = host.text.normalize_status(status),
        genres = genres,
    })
end

-- ─── ABI: get_chapter_list ─────────────────────────────────────────────────
-- arg: '"slug"'
function get_chapter_list(arg)
    local slug = host.json.decode(arg)
    if not slug then return host.json.encode({}) end

    log.info("mangahub chapters: slug=" .. slug)

    local gql = '{manga(x: ' .. SOURCE_ID .. ', slug: "' .. escape_gql(slug) .. '") {chapters {number, title, date}}}'

    local data = graphql_query(gql)
    local raw = {}
    if data and data.data and data.data.manga then
        raw = data.data.manga.chapters or {}
    end

    local chapters = {}
    for _, ch in ipairs(raw) do
        -- An absent title arrives as an empty string, not nil, so it has to be
        -- tested for emptiness: "" is truthy in Lua.
        local title = ch.title
        if not title or title == "" then title = "Chapter " .. tostring(ch.number) end

        local item = {
            id = slug .. ":chapter-" .. tostring(ch.number),
            manga_id = slug,
            title = title,
            chapter_num = host.text.chapter_num(ch.number),
            url = SITE_URL .. "/chapter/" .. slug .. "/chapter-" .. tostring(ch.number),
        }
        -- MangaHub sends epoch seconds, epoch millis or a date string; the host
        -- normalizes all three. An unparseable date leaves the key out so Go
        -- keeps its zero value instead of failing the decode.
        local iso = host.text.date_to_iso(ch.date)
        if iso ~= "" then item.released_at = iso end

        chapters[#chapters + 1] = item
    end

    -- GraphQL returns ascending; the ABI wants newest-first.
    table.sort(chapters, function(a, b) return a.chapter_num > b.chapter_num end)

    log.info("mangahub chapters: found " .. tostring(#chapters) .. " chapters for " .. slug)
    return host.json.encode(chapters)
end

-- ─── ABI: get_page_list ────────────────────────────────────────────────────
-- arg: '"slug:chapter-NUMBER"'
function get_page_list(arg)
    local chapterID = host.json.decode(arg)
    if not chapterID then return host.json.encode({}) end

    -- The trailing dash must be escaped: "r-" in a Lua pattern means "zero or
    -- more r, lazily", so a bare ":chapter-" would match ":chapte".
    local slug, number = chapterID:match("^(.-):chapter%-(.+)$")
    if not slug then return host.json.encode({}) end
    log.info("mangahub pages: slug=" .. slug .. " ch=" .. number)

    local gql = '{chapter(x: ' .. SOURCE_ID .. ', slug: "' .. escape_gql(slug)
        .. '", number: ' .. number .. ') {pages, mangaID, number}}'

    local data = graphql_query(gql)
    local chapter = data and data.data and data.data.chapter
    if not chapter then
        -- A refusal (rate limit, expired key) must fail loudly: returning an
        -- empty list renders a blank chapter with no explanation.
        if gqlError then error("mangahub: " .. gqlError) end
        return host.json.encode({})
    end

    local pagesJSON = host.json.decode(chapter.pages)
    if not pagesJSON then
        log.error("mangahub pages: unparseable pages JSON")
        return host.json.encode({})
    end

    local prefix = pagesJSON.p or ""
    local pages = {}
    for i, file in ipairs(pagesJSON.i or {}) do
        -- No per-page headers: the field must be left out, because an empty Lua
        -- table encodes as [] and the host decodes headers as a map.
        pages[#pages + 1] = {
            index = i - 1,
            url = IMG_CDN .. prefix .. file,
        }
    end

    log.info("mangahub pages: found " .. tostring(#pages) .. " pages for " .. chapterID)
    return host.json.encode(pages)
end

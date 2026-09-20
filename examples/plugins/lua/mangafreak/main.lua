-- Mangafreak plugin for goIsekai (Lua / Lunar VM)
-- Site: https://www.mangafreak.me (standalone HTML scraping, NOT Madara)
-- ABI contract version: 1

PLUGIN = {
    contract_version = 1,
    name = "Mangafreak",
    site_url = "https://www.mangafreak.me",
    logo = "logo.png",
    verify_url = "https://www.mangafreak.me",
    needs_human_verify = false,
    thumb_ratio = 0.703,
}

local BASE = "https://www.mangafreak.me"
local util = require("util")

-- ─── ABI: search_manga ─────────────────────────────────────────────────────
-- arg: {"query":"...","page":1}
-- Search URL: GET /Find/QUERY (path-based, NOT query-param)
-- Results: div.manga_search_item or div.mangaka_search_item
-- No pagination support (hasNextPage always false in Kotlin source)

function search_manga(arg)
    local args = host.json.decode(arg)
    local query = args.query or ""
    local page = tonumber(args.page) or 1
    local genres = args.genres or {}
    log.debug("mangafreak search q=" .. query .. " genres=" .. tostring(#genres))

    -- Genre browsing: GET /Genre/<slug>[/<page>]; the archive page reuses the
    -- ranking_item layout, so util.parse_ranking handles it.
    if #genres > 0 then
        local url = BASE .. "/Genre/" .. host.text.url_encode(genres[1])
        if page > 1 then
            url = url .. "/" .. tostring(page)
        end
        local body = host.http.get_body(url)
        if not body then
            return host.json.encode({})
        end
        return host.json.encode(util.parse_ranking(body))
    end

    if query == "" then
        return host.json.encode({})
    end

    local url = BASE .. "/Find/" .. host.text.url_encode(query)
    local body = host.http.get_body(url)
    if not body then
        return host.json.encode({})
    end

    local results = util.parse_search(body)
    log.debug("mangafreak search: found " .. #results .. " results for q=" .. query)
    return host.json.encode(results)
end

-- ─── ABI: get_manga_detail ─────────────────────────────────────────────────
-- arg: '"slug"' (JSON-encoded plain string)
-- Detail page: GET /manga/SLUG
-- Returns: {id, title, author, artist, description, cover_url, genres, status}

function get_manga_detail(arg)
    local manga_id = host.json.decode(arg)
    local body = host.http.get_body(BASE .. "/Manga/" .. manga_id)
    if not body then
        return host.json.encode({ id = manga_id })
    end
    return host.json.encode(util.parse_manga_detail(body, manga_id))
end

-- ─── ABI: get_chapter_list ─────────────────────────────────────────────────
-- arg: '"slug"' (JSON-encoded plain string)
-- Chapters are on the same page as detail: GET /manga/SLUG
-- Table rows in div.manga_series_list: td:eq(0)=name, td:eq(1)=date
-- Reversed to newest-first in util.parse_chapter_list

function get_chapter_list(arg)
    local manga_id = host.json.decode(arg)
    local body = host.http.get_body(BASE .. "/Manga/" .. manga_id)
    if not body then
        return host.json.encode({})
    end
    local chapters = util.parse_chapter_list(body, manga_id)
    log.debug("mangafreak chapters slug=" .. manga_id .. " count=" .. tostring(#chapters))
    return host.json.encode(chapters)
end

-- ─── ABI: get_page_list ────────────────────────────────────────────────────
-- arg: '"SLUG:chapter-PATH"' (JSON-encoded, colon = slash)
-- Chapter page: GET /manga/SLUG/chapter-PATH
-- Images: img#gohere[src] — all img with id="gohere" and src attribute

function get_page_list(arg)
    local chapter_id = host.json.decode(arg) -- "SLUG:chapter-PATH"
    -- Extract Read1_ path from "SLUG:Read1_SLUG_CHNUM"
    local ch_path = host.regex.find(chapter_id, [[:(.+)$]])
    local url = BASE .. "/Read1_" .. ch_path
    local body = host.http.get_body(url)
    if not body then
        return host.json.encode({})
    end
    local pages = util.parse_page_list(body)
    log.debug("mangafreak pages " .. chapter_id .. " count=" .. tostring(#pages))
    return host.json.encode(pages)
end


-- ─── get_genres (optional export) ──────────────────────────────────────────
-- Slugs as they appear in /Genre/<slug> (from the site's genre nav).
local GENRES = {
    { name = "Action",        slug = "Action" },
    { name = "Adult",         slug = "Adult" },
    { name = "Adventure",     slug = "Adventure" },
    { name = "Comedy",        slug = "Comedy" },
    { name = "Demons",        slug = "Demons" },
    { name = "Drama",         slug = "Drama" },
    { name = "Ecchi",         slug = "Ecchi" },
    { name = "Fantasy",       slug = "Fantasy" },
    { name = "Full Color",    slug = "Full_Color" },
    { name = "Gender Bender", slug = "Gender_Bender" },
    { name = "Harem",         slug = "Harem" },
    { name = "Historical",    slug = "Historical" },
    { name = "Horror",        slug = "Horror" },
    { name = "Isekai",        slug = "Isekai" },
    { name = "Josei",         slug = "Josei" },
    { name = "Magic",         slug = "Magic" },
    { name = "Manhwa",        slug = "Manhwa" },
    { name = "Martial Arts",  slug = "Martial_Arts" },
    { name = "Mature",        slug = "Mature" },
    { name = "Mecha",         slug = "Mecha" },
    { name = "Military",      slug = "Military" },
    { name = "Mystery",       slug = "Mystery" },
    { name = "One Shot",      slug = "One_Shot" },
    { name = "Psychological", slug = "Psychological" },
    { name = "Reincarnation", slug = "Reincarnation" },
    { name = "Romance",       slug = "Romance" },
    { name = "School Life",   slug = "School_Life" },
    { name = "Sci Fi",        slug = "Sci_Fi" },
    { name = "Seinen",        slug = "Seinen" },
    { name = "Shoujo",        slug = "Shoujo" },
    { name = "Shoujo Ai",     slug = "Shoujo_Ai" },
    { name = "Shounen",       slug = "Shounen" },
    { name = "Shounen Ai",    slug = "Shounen_Ai" },
    { name = "Slice Of Life", slug = "Slice_Of_Life" },
    { name = "Smut",          slug = "Smut" },
    { name = "Sports",        slug = "Sports" },
    { name = "Super Power",   slug = "Super_Power" },
    { name = "Supernatural",  slug = "Supernatural" },
    { name = "Tragedy",       slug = "Tragedy" },
    { name = "Vampire",       slug = "Vampire" },
    { name = "Webtoon",       slug = "Webtoon" },
    { name = "Yaoi",          slug = "Yaoi" },
    { name = "Yuri",          slug = "Yuri" },
}

function get_genres()
    return host.json.encode(GENRES)
end
